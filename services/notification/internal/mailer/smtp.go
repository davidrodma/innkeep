// Package mailer envia e-mails por SMTP.
//
// SMTP é o denominador comum: funciona com o Mailpit (servidor de testes
// local, que só captura as mensagens), com o Amazon SES em produção e com
// qualquer outro provedor — sem prender o projeto a um SDK específico.
package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/davidrodma/innkeep/pkg/kernel"
)

// Message é um e-mail a enviar.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// Sender envia e-mails. É a porta que o notificador usa; o adaptador SMTP
// abaixo é a implementação real, e os testes usam uma falsa.
type Sender interface {
	Send(ctx context.Context, message Message) error
}

// SMTPConfig configura o envio.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	TLS      string // "starttls", "tls" ou "none"
	From     string
}

// SMTP é o adaptador de saída para um servidor SMTP.
type SMTP struct {
	cfg SMTPConfig
}

// NewSMTP cria o adaptador.
func NewSMTP(cfg SMTPConfig) *SMTP { return &SMTP{cfg: cfg} }

// Send entrega a mensagem ao servidor SMTP.
func (s *SMTP) Send(ctx context.Context, message Message) error {
	from, err := mail.ParseAddress(s.cfg.From)
	if err != nil {
		return fmt.Errorf("mailer: remetente inválido (MAIL_FROM): %w", err)
	}
	to, err := mail.ParseAddress(message.To)
	if err != nil {
		// Endereço inválido não se resolve tentando de novo.
		return kernel.Invalid("mail.invalid_recipient", "destinatário de e-mail inválido")
	}

	address := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tlsConfig := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}

	var conn net.Conn
	if s.cfg.TLS == "tls" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("mailer: conectando a %s: %w", address, err)
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close() // a sessão nem começou: o erro que importa é o de cima
		return fmt.Errorf("mailer: iniciando sessão SMTP: %w", err)
	}
	defer client.Close()

	if s.cfg.TLS == "starttls" {
		// Sem TLS, usuário, senha e o conteúdo dos e-mails (com dados dos
		// hóspedes) trafegariam em texto puro. Se o servidor não oferecer
		// STARTTLS, desistimos em vez de enviar sem proteção.
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("mailer: o servidor %s não oferece STARTTLS", s.cfg.Host)
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("mailer: STARTTLS: %w", err)
		}
	}
	if s.cfg.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("mailer: autenticação SMTP: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("mailer: MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("mailer: RCPT TO: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA: %w", err)
	}
	if _, err := writer.Write(build(from, to, message)); err != nil {
		return fmt.Errorf("mailer: enviando corpo: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("mailer: finalizando mensagem: %w", err)
	}
	return client.Quit()
}

// build monta a mensagem MIME com as versões texto e HTML ("multipart/
// alternative"): o cliente de e-mail escolhe a que consegue exibir.
func build(from, to *mail.Address, message Message) []byte {
	const boundary = "innkeep-boundary-7d3f1a"
	var buffer bytes.Buffer

	header := func(name, value string) { fmt.Fprintf(&buffer, "%s: %s\r\n", name, value) }
	header("From", from.String())
	header("To", to.String())
	// QEncoding protege o assunto com acentos e, de quebra, impede injeção de
	// cabeçalhos via quebras de linha.
	header("Subject", mime.QEncoding.Encode("utf-8", sanitizeHeader(message.Subject)))
	header("Date", time.Now().Format(time.RFC1123Z))
	header("Message-ID", fmt.Sprintf("<%s@innkeep>", kernel.NewID()))
	header("MIME-Version", "1.0")
	header("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	buffer.WriteString("\r\n")

	part := func(contentType, body string) {
		fmt.Fprintf(&buffer, "--%s\r\nContent-Type: %s; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n", boundary, contentType)
		encoded := base64.StdEncoding.EncodeToString([]byte(body))
		for len(encoded) > 76 {
			buffer.WriteString(encoded[:76] + "\r\n")
			encoded = encoded[76:]
		}
		buffer.WriteString(encoded + "\r\n")
	}
	part("text/plain", message.Text)
	part("text/html", message.HTML)
	fmt.Fprintf(&buffer, "--%s--\r\n", boundary)
	return buffer.Bytes()
}

func sanitizeHeader(value string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
}
