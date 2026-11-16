# Política de segurança

## Como reportar uma vulnerabilidade

**Não abra uma issue pública.** Use o recurso *Report a vulnerability* (GitHub Security Advisories) na
aba *Security* do repositório:
<https://github.com/davidrodma/innkeep/security/advisories/new>. O relato fica privado entre você e os
mantenedores. Responderemos em até 5 dias úteis com a avaliação inicial e combinaremos a divulgação
depois da correção.

Inclua: versão/commit afetado, passos para reproduzir, impacto e, se possível, uma sugestão de correção.

## Versões com suporte

Somente a versão mais recente da branch `main` recebe correções de segurança.

## Modelo de segurança

| Ameaça | Defesa |
|---|---|
| Roubo de senhas do banco | Argon2id (19 MiB, 2 iterações); nunca texto puro |
| Força bruta no login | Bloqueio por e-mail após 5 falhas/15 min; limite por IP no gateway |
| Enumeração de usuários | Mesma mensagem e mesmo tempo de resposta para e-mail inexistente e senha errada |
| Forjar tokens | JWT Ed25519; chave privada só no `identity`; apenas `EdDSA` aceito |
| Token roubado | Vida de 15 min; refresh com rotação e detecção de reuso; logout revoga na hora |
| XSS roubando a sessão | Tokens em cookies `httpOnly`; HTML escapado pelo React e pelo `html/template` |
| CSRF | Cookies `SameSite=Lax`; Server Actions do Next.js verificam a origem |
| Escalada de privilégio | RBAC por permissão + restrição por propriedade, verificadas **antes** de alterar |
| Vazamento de dados pessoais (LGPD) | Documentos cifrados (AES-256-GCM) com índice cego; eventos e logs sem dados sensíveis; mascaramento em listagens; anonimização |
| SQL injection | Consultas 100% parametrizadas |
| SSRF por URLs cadastradas | Bloqueio de IPs internos/metadados no momento da conexão |
| Requisições repetidas | `Idempotency-Key` em criações e pagamentos |
| Abuso de API | Rate limit (janela deslizante no Redis); corpo limitado a 1 MB; timeouts do servidor |
| Contêiner comprometido | Sem root, FS somente leitura, sem capabilities, `seccomp`, NetworkPolicy "negar por padrão" |
| Segredos no Git | `.env` ignorado; segredos no AWS Secrets Manager; gitleaks no CI |
| Dependências vulneráveis | govulncheck, npm audit, Trivy nas imagens, Dependabot |
| Credenciais da nuvem no CI | OIDC com Role restrito à `main`/environments; nada guardado no GitHub |

## Chaves de desenvolvimento

O repositório contém **chaves de exemplo públicas** usadas apenas para o ambiente local subir sem
configuração (`fieldcrypt.DevelopmentKey`, senhas do `.env.example`). Os serviços **recusam iniciar em
produção** (`APP_ENV=production`) sem chaves próprias. Nunca reutilize esses valores.
