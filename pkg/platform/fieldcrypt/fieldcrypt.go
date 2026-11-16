// Package fieldcrypt criptografa campos sensíveis antes de gravá-los no banco
// (criptografia em nível de aplicação).
//
// Por que, se o disco do banco já é criptografado (RDS, por exemplo)? A
// criptografia de disco só protege contra o roubo do disco físico. Quem tem
// acesso de leitura ao banco — um backup vazado, uma consulta indevida, uma
// SQL injection — lê tudo em texto puro. Criptografando o CAMPO na aplicação,
// o banco guarda apenas bytes ilegíveis, e a chave vive em outro lugar (cofre
// de segredos). É uma exigência prática da LGPD para dados como CPF e
// passaporte, e é também como guardamos as credenciais dos canais de venda.
package fieldcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// DevelopmentKey é uma chave PÚBLICA, usada apenas para o projeto subir em
// desenvolvimento sem nenhuma configuração. Ela está no repositório: não
// protege nada. Por isso ForEnvironment se recusa a usá-la em produção.
//
// O comentário "gitleaks:allow" diz ao scanner de segredos do CI que esta
// linha é intencional. Ele vale SÓ para esta linha: qualquer outra chave que
// aparecer no código continua barrando o pull request.
const DevelopmentKey = "aW5ua2VlcC1kZXYta2V5LWRvLW5vdC11c2UtcHJvZCE=" // gitleaks:allow

// ForEnvironment cria o cifrador com a chave informada. Sem chave (ou com a
// chave de desenvolvimento), só funciona fora de produção — e devolve
// usingDevKey=true para o serviço registrar um aviso.
func ForEnvironment(key string, production bool) (c *Cipher, usingDevKey bool, err error) {
	if key == "" || key == DevelopmentKey {
		if production {
			return nil, false, errors.New("fieldcrypt: chave de criptografia obrigatória em produção (gere com `openssl rand -base64 32`)")
		}
		key, usingDevKey = DevelopmentKey, true
	}
	c, err = New(key)
	return c, usingDevKey, err
}

// Cipher cifra e decifra valores com AES-256-GCM.
//
// GCM é um modo "autenticado" (AEAD): além de esconder o conteúdo, detecta
// qualquer adulteração — um byte alterado no banco faz a decifragem falhar em
// vez de devolver lixo silenciosamente.
type Cipher struct {
	aead     cipher.AEAD
	indexKey []byte
}

// New cria o cifrador a partir de uma chave mestra de 32 bytes em base64
// (gere com `openssl rand -base64 32`).
//
// Da chave mestra são derivadas duas subchaves independentes — uma para
// cifrar e outra para o índice cego — para que o uso de uma não enfraqueça a
// outra (separação de chaves).
func New(masterKeyBase64 string) (*Cipher, error) {
	master, err := base64.StdEncoding.DecodeString(masterKeyBase64)
	if err != nil || len(master) != 32 {
		return nil, errors.New("fieldcrypt: a chave deve ter 32 bytes codificados em base64 (openssl rand -base64 32)")
	}
	block, err := aes.NewCipher(derive(master, "encryption"))
	if err != nil {
		return nil, fmt.Errorf("fieldcrypt: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("fieldcrypt: %w", err)
	}
	return &Cipher{aead: aead, indexKey: derive(master, "blind-index")}, nil
}

func derive(master []byte, purpose string) []byte {
	mac := hmac.New(sha256.New, master)
	mac.Write([]byte("innkeep/fieldcrypt/" + purpose))
	return mac.Sum(nil)
}

// Encrypt cifra o texto. O resultado é nonce || texto cifrado.
//
// O nonce (número usado uma única vez) é aleatório a cada chamada: cifrar o
// mesmo CPF duas vezes produz bytes diferentes, então não dá para descobrir
// que dois hóspedes têm o mesmo documento comparando as colunas.
func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	if plaintext == "" {
		return nil, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("fieldcrypt: gerando nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt decifra um valor produzido por Encrypt.
func (c *Cipher) Decrypt(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	size := c.aead.NonceSize()
	if len(data) < size {
		return "", errors.New("fieldcrypt: dado cifrado truncado")
	}
	plaintext, err := c.aead.Open(nil, data[:size], data[size:], nil)
	if err != nil {
		return "", errors.New("fieldcrypt: falha ao decifrar (chave errada ou dado adulterado)")
	}
	return string(plaintext), nil
}

// BlindIndex devolve um "índice cego": um HMAC do valor.
//
// Como o campo cifrado muda a cada gravação, não dá para fazer
// `WHERE documento = ?`. O índice cego resolve: o mesmo valor produz sempre o
// mesmo HMAC, que é gravado em uma coluna à parte e indexado. Busca-se pelo
// HMAC; e, sem a chave, ninguém consegue reverter nem testar valores em massa.
func (c *Cipher) BlindIndex(value string) string {
	if value == "" {
		return ""
	}
	mac := hmac.New(sha256.New, c.indexKey)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
