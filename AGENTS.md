# Instruções para agentes de IA

Este repositório usa o [CLAUDE.md](CLAUDE.md) como fonte principal de instruções para assistentes de
código (Claude Code, Codex, Cursor e similares). Leia-o primeiro, junto com o [MEMORY.md](MEMORY.md).

Resumo:

- Código em inglês; comentários, docs e textos da interface em pt-BR, explicando o porquê.
- Microserviços em Go com arquitetura hexagonal; comunicação entre serviços **apenas por eventos**
  (Kafka, outbox/inbox); banco por serviço.
- Antes de concluir uma tarefa: `./scripts/go-docker.sh sh -c 'gofmt -l . ; go vet ./... && go test ./...'`
  e, se mexeu no front-end, `cd web && npm run lint && npm run typecheck && npm test`.
- Regras detalhadas por área: [.claude/rules/](.claude/rules/).
