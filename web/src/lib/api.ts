import "server-only";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { ACCESS_COOKIE } from "./session-cookies";

// Este módulo só roda no SERVIDOR do Next.js (o import "server-only" quebra o
// build se alguém tentar usá-lo em um componente de cliente).
//
// O navegador nunca fala com a API do InnKeep diretamente: ele fala com o
// servidor do Next.js, que chama a API pela rede interna e acrescenta o token
// guardado em um cookie httpOnly. É o padrão BFF (Backend for Frontend) — o
// token de acesso jamais fica ao alcance de JavaScript rodando na página.

const API_URL = process.env.API_URL ?? "http://localhost:8080";

/** Erro devolvido pela API no formato Problem Details (RFC 9457). */
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
    /** Erros por campo, para marcar os inputs do formulário. */
    public readonly fields: Record<string, string> = {},
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type Options = {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  /** Rota pública: não envia token nem redireciona para o login. */
  anonymous?: boolean;
  /** Token explícito (usado logo após o login, antes de o cookie existir). */
  token?: string;
  /** Torna um POST seguro para repetição (veja o middleware no back-end). */
  idempotencyKey?: string;
};

export async function api<T>(path: string, options: Options = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };

  let token = options.token;
  if (!token && !options.anonymous) {
    token = (await cookies()).get(ACCESS_COOKIE)?.value;
  }
  if (token) headers.Authorization = `Bearer ${token}`;
  if (options.body !== undefined) headers["Content-Type"] = "application/json";
  if (options.idempotencyKey) headers["Idempotency-Key"] = options.idempotencyKey;

  let response: Response;
  try {
    response = await fetch(API_URL + path, {
      method: options.method ?? "GET",
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      // Dados operacionais (reservas, caixa) mudam o tempo todo: sem cache.
      cache: "no-store",
    });
  } catch {
    throw new ApiError(503, "network", "Não foi possível falar com o servidor. Tente novamente em instantes.");
  }

  if (response.status === 401 && !options.anonymous && !options.token) {
    // Sessão expirada ou revogada: volta para o login.
    redirect("/login?expired=1");
  }
  if (response.status === 204) return undefined as T;

  const text = await response.text();
  let data: Record<string, unknown> | null = null;
  try {
    data = text ? (JSON.parse(text) as Record<string, unknown>) : null;
  } catch {
    data = null;
  }

  if (!response.ok) {
    throw new ApiError(
      response.status,
      typeof data?.code === "string" ? data.code : "error",
      typeof data?.detail === "string" ? data.detail : "Ocorreu um erro inesperado.",
      (data?.errors as Record<string, string> | undefined) ?? {},
    );
  }
  return data as T;
}

/** Monta uma query string ignorando valores vazios. */
export function query(params: Record<string, string | number | boolean | undefined | null>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") search.set(key, String(value));
  }
  const text = search.toString();
  return text ? `?${text}` : "";
}
