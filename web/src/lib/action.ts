import "server-only";

import { unstable_rethrow } from "next/navigation";

import type { ActionState } from "./action-state";
import { ApiError } from "./api";
import { parseMoney } from "./format";

export type { ActionState };

/**
 * Converte uma exceção no estado que o formulário entende.
 *
 * As mensagens de validação vêm do BACK-END: é ele a autoridade sobre as
 * regras de negócio. O front-end não duplica essas regras — apenas mostra o
 * que a API respondeu, campo a campo.
 */
export function failure(error: unknown): ActionState {
  // redirect() e notFound() funcionam lançando exceções especiais; elas
  // precisam continuar subindo, e não virar mensagem de erro.
  unstable_rethrow(error);
  if (error instanceof ApiError) {
    return { ok: false, message: error.message, errors: error.fields };
  }
  console.error(error);
  return { ok: false, message: "Ocorreu um erro inesperado. Tente novamente." };
}

/** Lê um campo de texto do formulário, sem espaços nas pontas. */
export function text(form: FormData, name: string): string {
  const value = form.get(name);
  return typeof value === "string" ? value.trim() : "";
}

/** Lê um campo numérico inteiro. */
export function integer(form: FormData, name: string, fallback = 0): number {
  const value = Number.parseInt(text(form, name), 10);
  return Number.isFinite(value) ? value : fallback;
}

/** Lê um campo de valor em reais e devolve centavos (null se vazio ou inválido). */
export function money(form: FormData, name: string): number | null {
  const value = text(form, name);
  return value === "" ? null : parseMoney(value);
}

export function checked(form: FormData, name: string): boolean {
  return form.get(name) === "on" || form.get(name) === "true";
}
