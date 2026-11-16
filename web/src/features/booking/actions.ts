"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { failure, integer, text, type ActionState } from "@/lib/action";
import { api } from "@/lib/api";
import { BOOKING_COOKIE, cookieOptions } from "@/lib/session-cookies";
import type { PublicReservation } from "@/lib/types";

/**
 * Lembra, em um cookie httpOnly, o par localizador + e-mail das reservas que
 * o visitante fez neste navegador. Assim a página de acompanhamento abre
 * direto, sem pedir o e-mail de novo — e sem o e-mail aparecer na URL.
 */
async function rememberBooking(code: string, email: string) {
  const store = await cookies();
  let saved: Record<string, string> = {};
  try {
    saved = JSON.parse(store.get(BOOKING_COOKIE)?.value ?? "{}") as Record<string, string>;
  } catch {
    saved = {};
  }
  saved[code] = email;
  const recent = Object.fromEntries(Object.entries(saved).slice(-5));
  store.set(BOOKING_COOKIE, JSON.stringify(recent), cookieOptions(new Date(Date.now() + 90 * 86_400_000)));
}

export async function bookStay(_previous: ActionState, form: FormData): Promise<ActionState> {
  const email = text(form, "email").toLowerCase();
  if (text(form, "accept_terms") !== "on") {
    return { ok: false, message: "Para continuar, aceite a política de cancelamento.", errors: { accept_terms: "Obrigatório" } };
  }

  let reservation: PublicReservation;
  try {
    reservation = await api<PublicReservation>("/v1/public/reservations", {
      method: "POST",
      anonymous: true,
      idempotencyKey: text(form, "idempotency_key"),
      body: {
        property_id: text(form, "property_id"),
        room_type_id: text(form, "room_type_id"),
        check_in: text(form, "check_in"),
        check_out: text(form, "check_out"),
        adults: integer(form, "adults", 1),
        children: integer(form, "children", 0),
        holder: { name: text(form, "name"), email, phone: text(form, "phone") },
        notes: text(form, "notes"),
      },
    });
  } catch (error) {
    return failure(error);
  }
  await rememberBooking(reservation.code, email);
  redirect(`/booking/${reservation.code}`);
}

export async function lookupBooking(_previous: ActionState, form: FormData): Promise<ActionState> {
  const code = text(form, "code").toUpperCase();
  const email = text(form, "email").toLowerCase();
  try {
    await api<PublicReservation>("/v1/public/reservations/lookup", { method: "POST", anonymous: true, body: { code, email } });
  } catch (error) {
    const result = failure(error);
    return { ...result, message: "Não encontramos uma reserva com este localizador e e-mail." };
  }
  await rememberBooking(code, email);
  redirect(`/booking/${code}`);
}
