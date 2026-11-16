# Memória do projeto

Registro curto de fatos aprendidos durante o desenvolvimento que **não** são óbvios lendo o código e
que evitam repetir erros. Mantenha cada item com o *porquê*. Remova o que deixar de ser verdade.

## Armadilhas já resolvidas (não reintroduzir)

- **`errors.Is` com erros de domínio**: `kernel.Error.Wrap`/`WithField` devolvem cópias; por isso
  `*kernel.Error` implementa `Is` (compara Kind + Code). Sem isso, `errors.Is(err, eventstore.ErrConcurrency)`
  falhava e o serviço de reservas nunca repetia comandos em conflito. Achado por teste de integração.
- **"Hoje" depende do fuso**: o domínio calcula a data corrente no fuso da propriedade. Testes que
  montam datas devem usar `America/Sao_Paulo` (às 22h em SP já é amanhã em UTC).
- **Ordem entre tópicos não é garantida**: um consumidor de `channel.sync_requested` pode receber o
  pedido antes de `property.room_type_defined`. Handlers de tópicos diferentes devem tolerar a ordem
  inversa (ex.: `Resync` ignora tipo ainda desconhecido em vez de ir para a DLQ).
- **UUID v7 monotônico**: `kernel.NewID` usa contador no mesmo milissegundo (RFC 9562, método 1).
- **Reservas de canal**: ID determinístico = UUID v5 de `propriedade:canal:id-externo` (o número da
  reserva só é único dentro da conta do canal).
- **Outbox**: o relay publica antes de marcar `published_at`; testes devem esperar a outbox esvaziar,
  não só o broker receber.
- **React keys**: listas dinâmicas no front (formas de pagamento) usam contador em `useRef`.
- **Trace no consumidor Kafka**: `kotel.WithProcessSpan` devolve um contexto derivado da mensagem,
  **sem** o cancelamento do consumidor. O span é anexado ao `ctx` recebido (`trace.ContextWithSpan`);
  sem isso, o desligamento esperava todas as novas tentativas. Achado pelo staticcheck (SA4009).

## Ambiente e ferramentas

- Next.js **16**: `middleware.ts` virou **`proxy.ts`**; `params`, `searchParams` e `cookies()` são async.
  Há documentação local em `web/node_modules/next/dist/docs/`.
- TypeScript fixado em **6.0.x**: o `typescript-eslint` (via `eslint-config-next`) suporta `<6.1`.
- Se o `npm install` "perder" binários nativos opcionais (rolldown/swc), apague `node_modules` e
  `package-lock.json` e reinstale (bug do npm com dependências opcionais).
- Imagem final dos serviços é distroless: o healthcheck do Compose é `/app healthcheck`.
- `go test -race` exige cgo: use a imagem `golang` (Debian), não a alpine.
- Kafka local em KRaft (`apache/kafka:4.1.0`); PostgreSQL 18 monta o volume em `/var/lib/postgresql`.
- **Versão do Go no CI**: `GO_VERSION` (1.27, igual ao `ARG` do Dockerfile) com `check-latest`. Não
  volte para `go-version-file: go.mod`: o go.mod tem só a versão mínima (1.26.0), o setup-go instalaria
  exatamente ela e o `govulncheck` acusaria falhas da biblioteca padrão já corrigidas.
- **golangci-lint v2**: `misspell` desligado (só conhece inglês; os comentários são em pt-BR); erros de
  `Close`/`Print` dispensados pelo preset `std-error-handling`; exceções do gosec só na linha, com motivo.
- **gitleaks**: a chave de desenvolvimento do `fieldcrypt` é marcada com `gitleaks:allow`; qualquer
  outra chave no código reprova o CI. O `.gitleaks.toml` só ignora pastas geradas (`node_modules`...).

## Integrações externas

- Booking.com Connectivity API (consultada em nov/2026): exige parceiro certificado; token via
  `connectivity-authentication.booking.com`; hosts `supply-xml` (ARI) e `secure-supply-xml` (reservas);
  `End` é **inclusivo** nas mensagens de disponibilidade/preço. Check-out e distinção
  cancelamento/modificação foram **inferidos** — validar na certificação.
- Airbnb: API só para parceiros homologados; usar iCal (ignorar eventos "Not available" para não ecoar).
