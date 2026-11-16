package notifier

// Modelos dos e-mails. Estilos embutidos (inline) de propósito: clientes de
// e-mail ignoram folhas de estilo externas e boa parte do CSS moderno.

const htmlLayout = `<!doctype html>
<html lang="pt-BR">
<body style="margin:0;padding:24px;background:#f4f5f7;font-family:Arial,Helvetica,sans-serif;color:#1f2933;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;margin:0 auto;background:#ffffff;border-radius:12px;">
    <tr><td style="padding:28px 28px 8px;">
      <p style="margin:0 0 4px;font-size:13px;color:#667085;">{{.PropertyName}}</p>
      <h1 style="margin:0 0 12px;font-size:22px;line-height:1.3;">{{.Heading}}</h1>
      <p style="margin:0 0 20px;font-size:15px;line-height:1.5;">Olá, {{.GuestName}}. {{.Intro}}</p>
    </td></tr>
    <tr><td style="padding:0 28px;">
      <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="font-size:14px;border-top:1px solid #e4e7ec;">
        <tr><td style="padding:10px 0;color:#667085;">Localizador</td><td style="padding:10px 0;text-align:right;font-weight:bold;">{{.Code}}</td></tr>
        <tr><td style="padding:10px 0;color:#667085;border-top:1px solid #f0f2f5;">Entrada</td><td style="padding:10px 0;text-align:right;border-top:1px solid #f0f2f5;">{{.CheckIn}}{{if .CheckInTime}}, a partir das {{.CheckInTime}}{{end}}</td></tr>
        <tr><td style="padding:10px 0;color:#667085;border-top:1px solid #f0f2f5;">Saída</td><td style="padding:10px 0;text-align:right;border-top:1px solid #f0f2f5;">{{.CheckOut}}{{if .CheckOutTime}}, até as {{.CheckOutTime}}{{end}}</td></tr>
        <tr><td style="padding:10px 0;color:#667085;border-top:1px solid #f0f2f5;">Hóspedes</td><td style="padding:10px 0;text-align:right;border-top:1px solid #f0f2f5;">{{.Guests}}</td></tr>
        {{if .RoomName}}<tr><td style="padding:10px 0;color:#667085;border-top:1px solid #f0f2f5;">Acomodação</td><td style="padding:10px 0;text-align:right;border-top:1px solid #f0f2f5;">{{.RoomName}}</td></tr>{{end}}
        <tr><td style="padding:10px 0;color:#667085;border-top:1px solid #f0f2f5;">Total</td><td style="padding:10px 0;text-align:right;border-top:1px solid #f0f2f5;font-weight:bold;">{{.Total}}</td></tr>
      </table>
    </td></tr>
    {{if .ShowPayment}}
    <tr><td style="padding:20px 28px 0;">
      <div style="background:#fff8e6;border:1px solid #f5d98b;border-radius:8px;padding:16px;font-size:14px;line-height:1.5;">
        <strong>Sinal para confirmar: {{.Deposit}}</strong><br>
        {{if .PixKey}}Chave Pix: <strong>{{.PixKey}}</strong><br>{{end}}
        {{if .HoldExpiresAt}}Pague até {{.HoldExpiresAt}} (horário de Brasília). Depois disso a reserva expira.{{end}}
      </div>
    </td></tr>
    {{end}}
    <tr><td style="padding:24px 28px;">
      <a href="{{.StatusURL}}" style="display:inline-block;background:#0f766e;color:#ffffff;text-decoration:none;padding:12px 20px;border-radius:8px;font-size:14px;font-weight:bold;">Acompanhar minha reserva</a>
    </td></tr>
    <tr><td style="padding:16px 28px 28px;border-top:1px solid #e4e7ec;font-size:12px;line-height:1.6;color:#667085;">
      {{.PropertyName}}{{if .Address}}<br>{{.Address}}{{end}}{{if .Phone}}<br>{{.Phone}}{{end}}{{if .Email}}<br>{{.Email}}{{end}}
    </td></tr>
  </table>
</body>
</html>`

const textLayout = `{{.Heading}}

Olá, {{.GuestName}}. {{.Intro}}

Localizador: {{.Code}}
Entrada: {{.CheckIn}}{{if .CheckInTime}}, a partir das {{.CheckInTime}}{{end}}
Saída: {{.CheckOut}}{{if .CheckOutTime}}, até as {{.CheckOutTime}}{{end}}
Hóspedes: {{.Guests}}{{if .RoomName}}
Acomodação: {{.RoomName}}{{end}}
Total: {{.Total}}
{{if .ShowPayment}}
Sinal para confirmar: {{.Deposit}}{{if .PixKey}}
Chave Pix: {{.PixKey}}{{end}}{{if .HoldExpiresAt}}
Pague até {{.HoldExpiresAt}} (horário de Brasília). Depois disso a reserva expira.{{end}}
{{end}}
Acompanhe sua reserva: {{.StatusURL}}

{{.PropertyName}}{{if .Address}}
{{.Address}}{{end}}{{if .Phone}}
{{.Phone}}{{end}}{{if .Email}}
{{.Email}}{{end}}
`
