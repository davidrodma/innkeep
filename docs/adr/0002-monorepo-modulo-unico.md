# ADR 0002 — Monorepo com um único módulo Go

**Situação:** aceita

## Contexto

Microserviços costumam viver em repositórios separados, com versões independentes de bibliotecas
compartilhadas. Isso tem um custo alto para uma equipe pequena: atualizar o kit de plataforma exige
publicar versões e atualizar N repositórios.

## Decisão

- Um repositório com todos os serviços, o front-end e a infraestrutura.
- **Um único `go.mod`** (`github.com/davidrodma/innkeep`). Código compartilhado em `pkg/`; código de
  cada serviço em `services/<nome>/internal/`.
- O isolamento é garantido pelo **compilador**: um pacote `internal/` só pode ser importado por código
  da mesma árvore. Um serviço não consegue importar o domínio de outro, nem por acidente.
- Cada serviço continua sendo um **binário e uma imagem independentes**, implantados separadamente.

## Consequências

- (+) Uma mudança em um contrato de evento e em todos os consumidores entra em um único pull request.
- (+) Uma versão de cada dependência para todo o sistema.
- (−) O CI compila tudo a cada mudança (rápido em Go; pode ser otimizado por caminhos alterados).
- Para separar um serviço em outro repositório no futuro, basta mover a pasta e copiar `pkg/`.
