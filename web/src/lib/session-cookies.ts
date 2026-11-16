// Nomes e opções dos cookies de sessão. Fica em um módulo sem dependências de
// servidor porque também é importado pelo proxy.ts.

export const ACCESS_COOKIE = "ik_at"; // token de acesso (JWT), vida curta
export const REFRESH_COOKIE = "ik_rt"; // refresh token, vida longa
export const PROPERTY_COOKIE = "ik_prop"; // propriedade selecionada no painel
export const BOOKING_COOKIE = "ik_bk"; // reservas feitas pelo visitante no site público

/**
 * Opções comuns dos cookies de sessão:
 *
 *  - httpOnly: JavaScript da página não consegue ler o cookie. Se um script
 *    malicioso for injetado (XSS), ele não rouba a sessão.
 *  - secure: só trafega por HTTPS (desligado em desenvolvimento, que usa http).
 *  - sameSite "lax": o navegador não envia o cookie em requisições disparadas
 *    por outros sites (POST de formulário, fetch), o que barra ataques CSRF.
 */
export function cookieOptions(expires: Date) {
  return {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production" && !process.env.PUBLIC_WEB_URL?.startsWith("http://"),
    sameSite: "lax" as const,
    path: "/",
    expires,
  };
}

/** Lê a expiração (claim `exp`) de um JWT sem validar a assinatura. */
export function tokenExpiry(token: string): number {
  try {
    const payload = token.split(".")[1] ?? "";
    const json = Buffer.from(payload.replace(/-/g, "+").replace(/_/g, "/"), "base64").toString("utf8");
    const exp = (JSON.parse(json) as { exp?: number }).exp;
    return typeof exp === "number" ? exp * 1000 : 0;
  } catch {
    return 0;
  }
}
