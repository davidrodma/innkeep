// Package auth implementa a mecânica de autenticação por token nos serviços.
//
// Modelo:
//   - O serviço identity autentica o usuário e emite um JWT assinado com uma
//     chave PRIVADA Ed25519 que só ele possui (Signer).
//   - Todos os outros serviços validam o token com a chave PÚBLICA, obtida do
//     endpoint JWKS do identity (Verifier). Nenhum deles consegue forjar
//     tokens, e nenhum segredo compartilhado circula entre serviços.
//   - Cada serviço valida o token por conta própria, mesmo estando atrás do
//     gateway — "defesa em profundidade": não confiamos na rede interna.
//   - A autorização é por papéis (RBAC): o token carrega o papel do usuário e
//     cada rota exige uma permissão (Guard.Require). Papéis e permissões são
//     definidos em pkg/kernel/access.
package auth
