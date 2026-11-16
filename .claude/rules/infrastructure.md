---
paths:
  - "deploy/**"
  - ".github/**"
  - "docker-compose.yml"
---

# Regras de infraestrutura

- **Helm** (`deploy/helm/innkeep`): um bloco por serviço em `values.yaml`; segredos só por nome de
  chave (`secretEnv`), nunca valores. Contêineres sem root, FS somente leitura, sem capabilities.
  Migrações em Job de hook `pre-install,pre-upgrade`.
- **Terraform** (`deploy/terraform`): módulos oficiais `terraform-aws-modules` com versão fixada por
  `~>`; nada de segredo em variáveis — a aplicação lê do Secrets Manager. Recursos de dados privados,
  cifrados e acessíveis só pelo security group do EKS.
- Valide com `./scripts/validate-infra.sh` (helm lint/template + terraform fmt/validate em contêiner).
  Nunca rode `terraform apply`/`destroy` sem pedido explícito do usuário.
- **GitHub Actions**: `permissions` mínimas por workflow; AWS só por OIDC; actions de terceiros com
  versão fixada (idealmente por SHA, atualizadas pelo Dependabot).
- **Compose**: portas publicadas só em `127.0.0.1`; serviços Go com `read_only`, `cap_drop: [ALL]` e
  healthcheck `/app healthcheck`.
