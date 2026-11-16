import "server-only";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { cache } from "react";

import { api } from "./api";
import { ACCESS_COOKIE, PROPERTY_COOKIE, REFRESH_COOKIE, cookieOptions } from "./session-cookies";
import type { List, Property, Tokens, User } from "./types";

/** Grava os cookies de sessão após login ou renovação. */
export async function setSessionCookies(tokens: Tokens) {
  const store = await cookies();
  store.set(ACCESS_COOKIE, tokens.access_token, cookieOptions(new Date(tokens.expires_at)));
  store.set(REFRESH_COOKIE, tokens.refresh_token, cookieOptions(new Date(tokens.refresh_expires_at)));
}

export async function clearSessionCookies() {
  const store = await cookies();
  store.delete(ACCESS_COOKIE);
  store.delete(REFRESH_COOKIE);
}

/**
 * Usuário autenticado da requisição atual.
 *
 * `cache` (do React) memoriza o resultado DURANTE uma renderização: o layout e
 * a página podem chamar currentUser() à vontade, e a API é consultada uma vez.
 */
export const currentUser = cache(async (): Promise<User> => {
  return api<User>("/v1/auth/me");
});

export function can(user: User, permission: string): boolean {
  return user.permissions.includes(permission);
}

/** Propriedades que o usuário pode operar. */
export const availableProperties = cache(async (): Promise<Property[]> => {
  const list = await api<List<Property>>("/v1/properties");
  return list.items;
});

/**
 * Propriedade selecionada no painel. Se ainda não existe nenhuma, leva ao
 * cadastro da primeira.
 */
export const currentProperty = cache(async (): Promise<Property> => {
  const properties = await availableProperties();
  if (properties.length === 0) redirect("/admin/property/new");
  const selected = (await cookies()).get(PROPERTY_COOKIE)?.value;
  return properties.find((property) => property.id === selected) ?? properties[0]!;
});
