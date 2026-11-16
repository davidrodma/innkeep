"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { failure, text, type ActionState } from "@/lib/action";
import { api } from "@/lib/api";
import { clearSessionCookies, setSessionCookies } from "@/lib/session";
import { ACCESS_COOKIE, PROPERTY_COOKIE, REFRESH_COOKIE, cookieOptions } from "@/lib/session-cookies";
import type { Tokens } from "@/lib/types";

// "use server" transforma as funções exportadas em Server Actions: endpoints
// chamados pelos formulários, que rodam no servidor do Next.js. O navegador
// envia o FormData; credenciais e tokens nunca passam por código de cliente.

export async function login(_previous: ActionState, form: FormData): Promise<ActionState> {
  let tokens: Tokens;
  try {
    tokens = await api<Tokens>("/v1/auth/login", {
      method: "POST",
      anonymous: true,
      body: { email: text(form, "email"), password: String(form.get("password") ?? "") },
    });
  } catch (error) {
    return failure(error);
  }
  await setSessionCookies(tokens);

  // Só aceitamos voltar para caminhos internos do painel. Redirecionar para
  // qualquer URL recebida abriria a porta para phishing ("open redirect").
  const next = text(form, "next");
  redirect(next.startsWith("/admin") && !next.startsWith("//") ? next : "/admin");
}

export async function logout() {
  const store = await cookies();
  try {
    await api("/v1/auth/logout", {
      method: "POST",
      anonymous: true,
      token: store.get(ACCESS_COOKIE)?.value,
      body: { refresh_token: store.get(REFRESH_COOKIE)?.value ?? "" },
    });
  } catch {
    // Mesmo que a API falhe, a sessão local é encerrada.
  }
  await clearSessionCookies();
  redirect("/login");
}

/** Troca a propriedade em uso no painel. */
export async function selectProperty(form: FormData) {
  const store = await cookies();
  const oneYear = new Date(Date.now() + 365 * 86_400_000);
  store.set(PROPERTY_COOKIE, text(form, "property_id"), cookieOptions(oneYear));
  redirect("/admin");
}

export async function changePassword(_previous: ActionState, form: FormData): Promise<ActionState> {
  const newPassword = String(form.get("new_password") ?? "");
  if (newPassword !== String(form.get("confirm_password") ?? "")) {
    return { ok: false, message: "As senhas não conferem.", errors: { confirm_password: "Repita a nova senha" } };
  }
  try {
    await api("/v1/auth/password", {
      method: "PUT",
      body: { current_password: String(form.get("current_password") ?? ""), new_password: newPassword },
    });
  } catch (error) {
    return failure(error);
  }
  return { ok: true, message: "Senha alterada. As outras sessões foram encerradas." };
}
