# Como contribuir

Obrigado pelo interesse! Este guia resume as convenções do projeto.

## Idioma

- **Código em inglês**: nomes de variáveis, funções, tipos, arquivos, rotas, tabelas e eventos.
- **Comentários, documentação, mensagens de erro e textos da interface em português (pt-BR).**
- Comentários explicam o **porquê** (a intenção, o risco evitado), não o óbvio.

## Fluxo

1. Abra uma issue descrevendo o problema ou a proposta (para mudanças grandes, discuta antes).
2. Crie um branch a partir da `main`: `feat/...`, `fix/...`, `docs/...`.
3. Commits no formato [Conventional Commits](https://www.conventionalcommits.org/pt-br/):
   `feat(billing): permite devolução parcial`.
4. Abra o pull request; o CI precisa ficar verde (lint, testes, segurança, E2E).

## Checklist do pull request

- [ ] Testes para o comportamento novo (unitários no domínio; integração/E2E quando atravessa serviços).
- [ ] `gofmt`, `go vet`, `golangci-lint` e `npm run lint` sem avisos. Os comandos exatos, iguais aos do
      CI, estão em [.claude/skills/run-checks/SKILL.md](.claude/skills/run-checks/SKILL.md).
- [ ] Precisa abrir uma exceção num alerta do lint ou do scanner de segredos? Só na própria linha e com
      o motivo (`//nolint:gosec // ...`, `// gitleaks:allow`), nunca desligando a regra inteira.
- [ ] Mudou um evento? Só acréscimos compatíveis — ou um tipo `.v2` novo (veja `docs/architecture/events.md`).
- [ ] Mudou o banco? Nova migração em `services/<serviço>/migrations/`, **compatível com a versão anterior**
      do código (nunca edite uma migração já publicada).
- [ ] Nenhum dado pessoal em logs ou eventos; nenhum segredo no código.
- [ ] Rotas novas? Rode `python3 scripts/list-endpoints.py > docs/api.md`.
- [ ] Decisão de arquitetura relevante? Escreva um ADR em `docs/adr/`.

## Arquitetura — regras que o review vai cobrar

- `domain/` só importa `pkg/kernel` (e a biblioteca padrão).
- Casos de uso dependem de **interfaces** (portas) declaradas em `app/`; adaptadores ficam em `adapter/`.
- Serviços **nunca** importam o `internal/` de outro serviço nem leem o banco de outro.
- Toda escrita que gera evento usa `tx.WithinTx` + `publisher.Publish` (outbox) na mesma transação.
- Todo consumidor é idempotente (middleware `inbox` ou operação naturalmente idempotente).
- Dinheiro em centavos (`int64`); datas de estadia como `kernel.Date`.

## Ambiente

Veja [docs/guides/deploy-local.md](docs/guides/deploy-local.md). O projeto também traz instruções para
desenvolvimento com IA em [CLAUDE.md](CLAUDE.md) e [.claude/](.claude/).
