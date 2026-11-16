import { NextResponse, type NextRequest } from "next/server";

import { ACCESS_COOKIE, REFRESH_COOKIE, cookieOptions, tokenExpiry } from "@/lib/session-cookies";
import type { Tokens } from "@/lib/types";

// proxy.ts (o antigo middleware.ts) roda ANTES de cada requisição ao painel.
// Tem duas funções:
//
//  1. barrar quem não está autenticado, antes de qualquer página renderizar;
//  2. renovar a sessão em silêncio: o token de acesso vive só 15 minutos, e é
//     aqui que ele é trocado por um novo usando o refresh token — o usuário
//     não percebe nada.
//
// Atenção: isto é uma verificação "otimista" (o cookie existe e não expirou).
// A validação de verdade — assinatura do JWT e permissões — acontece na API, a
// cada chamada. Nunca confie só no proxy para proteger dados.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

/** Renova quando faltar menos que isto para o token expirar. */
const REFRESH_MARGIN_MS = 30_000;

export async function proxy(request: NextRequest) {
  const accessToken = request.cookies.get(ACCESS_COOKIE)?.value;
  const refreshToken = request.cookies.get(REFRESH_COOKIE)?.value;

  if (accessToken && tokenExpiry(accessToken) - Date.now() > REFRESH_MARGIN_MS) {
    return NextResponse.next();
  }
  if (!refreshToken) return toLogin(request);

  const tokens = await refresh(refreshToken);
  if (!tokens) return toLogin(request);

  // Os cookies novos precisam chegar a dois lugares: à requisição em curso
  // (para a página que vai renderizar agora) e ao navegador (para as próximas).
  request.cookies.set(ACCESS_COOKIE, tokens.access_token);
  request.cookies.set(REFRESH_COOKIE, tokens.refresh_token);
  const response = NextResponse.next({ request: { headers: request.headers } });
  response.cookies.set(ACCESS_COOKIE, tokens.access_token, cookieOptions(new Date(tokens.expires_at)));
  response.cookies.set(REFRESH_COOKIE, tokens.refresh_token, cookieOptions(new Date(tokens.refresh_expires_at)));
  return response;
}

async function refresh(refreshToken: string): Promise<Tokens | null> {
  try {
    const response = await fetch(`${API_URL}/v1/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
      cache: "no-store",
    });
    return response.ok ? ((await response.json()) as Tokens) : null;
  } catch {
    return null;
  }
}

function toLogin(request: NextRequest) {
  const url = new URL("/login", request.url);
  // Guarda para onde a pessoa queria ir, para voltar depois do login.
  url.searchParams.set("next", request.nextUrl.pathname + request.nextUrl.search);
  const response = NextResponse.redirect(url);
  response.cookies.delete(ACCESS_COOKIE);
  response.cookies.delete(REFRESH_COOKIE);
  return response;
}

export const config = {
  // Só o painel administrativo passa pelo proxy; o site público é aberto.
  matcher: ["/admin/:path*"],
};
