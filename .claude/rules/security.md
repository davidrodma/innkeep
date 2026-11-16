# Regras de segurança (valem para todo o repositório)

O repositório é **público**. Antes de concluir qualquer tarefa, confira:

- **Segredos**: nada de senha, token, chave ou URL com credencial no código, em testes, docs ou
  exemplos (exceto as chaves de desenvolvimento já documentadas, que os serviços recusam em produção).
  Configuração sensível vem de variáveis de ambiente / AWS Secrets Manager.
- **Dados pessoais (LGPD)**: não registre em logs nem coloque em eventos nome, e-mail, telefone, CPF,
  documento ou endereço. Use IDs. Dados de hóspedes ficam só no serviço `guest`, cifrados.
- **Autorização** verificada no servidor, antes de alterar: permissão da rota + `access.EnsureProperty`.
  Recurso de outra propriedade responde como "não encontrado" quando revelar sua existência for um risco.
- **Entradas**: valide no domínio; SQL parametrizado; corpo JSON estrito e limitado (`httpx.Decode`);
  URLs informadas por usuários só pelo cliente `safehttp` (proteção contra SSRF).
- **Saídas**: erros internos nunca vazam detalhes ao cliente (`httpx.Error` faz isso); e-mails com `html/template`.
- **Autenticação**: não crie rotas públicas novas sem necessidade; rotas públicas sensíveis passam
  pelo rate limit reforçado do gateway (`Sensitive: true`).
- **Dependências**: prefira a biblioteca padrão; dependência nova precisa de justificativa.
