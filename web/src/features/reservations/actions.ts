"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { checked, failure, integer, money, text, type ActionState } from "@/lib/action";
import { api } from "@/lib/api";
import type { Reservation } from "@/lib/types";

/** Cria uma reserva pelo painel (inicia a saga de alocação). */
export async function createReservation(_previous: ActionState, form: FormData): Promise<ActionState> {
  const quoted = money(form, "quoted_total");
  if (text(form, "quoted_total") !== "" && quoted === null) {
    return { ok: false, message: "Valor inválido.", errors: { quoted_total: "Use o formato 150,00" } };
  }

  let reservation: Reservation;
  try {
    reservation = await api<Reservation>("/v1/reservations", {
      method: "POST",
      // A chave nasce com o formulário: reenviar o MESMO formulário (clique
      // duplo, tecla F5) não cria uma segunda reserva.
      idempotencyKey: text(form, "idempotency_key"),
      body: {
        property_id: text(form, "property_id"),
        room_type_id: text(form, "room_type_id"),
        room_id: text(form, "room_id"),
        check_in: text(form, "check_in"),
        check_out: text(form, "check_out"),
        adults: integer(form, "adults", 1),
        children: integer(form, "children", 0),
        holder: { name: text(form, "holder_name"), email: text(form, "holder_email"), phone: text(form, "holder_phone") },
        notes: text(form, "notes"),
        quoted_total_cents: quoted,
        confirm_now: checked(form, "confirm_now"),
      },
    });
  } catch (error) {
    return failure(error);
  }
  redirect(`/admin/reservations/${reservation.id}`);
}

/** Ações simples sobre uma reserva: confirmar, check-in, check-out, no-show. */
export async function transition(_previous: ActionState, form: FormData): Promise<ActionState> {
  const id = text(form, "id");
  const command = text(form, "command");
  if (!["confirm", "check-in", "check-out", "no-show"].includes(command)) {
    return { ok: false, message: "Ação desconhecida." };
  }
  try {
    await api(`/v1/reservations/${id}/${command}`, { method: "POST" });
  } catch (error) {
    return failure(error);
  }
  revalidatePath(`/admin/reservations/${id}`);
  return { ok: true };
}

export async function cancelReservation(_previous: ActionState, form: FormData): Promise<ActionState> {
  const id = text(form, "id");
  try {
    await api(`/v1/reservations/${id}/cancel`, { method: "POST", body: { reason: text(form, "reason") } });
  } catch (error) {
    return failure(error);
  }
  revalidatePath(`/admin/reservations/${id}`);
  return { ok: true, message: "Reserva cancelada." };
}

export async function changeStay(_previous: ActionState, form: FormData): Promise<ActionState> {
  const id = text(form, "id");
  const quoted = money(form, "quoted_total");
  try {
    await api(`/v1/reservations/${id}/change-stay`, {
      method: "POST",
      body: {
        check_in: text(form, "check_in"),
        check_out: text(form, "check_out"),
        room_id: text(form, "room_id"),
        quoted_total_cents: quoted,
      },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath(`/admin/reservations/${id}`);
  return { ok: true, message: "Alteração solicitada. O inventário está verificando a disponibilidade." };
}

// ---- Conta (folio) ----

export async function recordPayments(_previous: ActionState, form: FormData): Promise<ActionState> {
  const reservationId = text(form, "reservation_id");
  const methods = form.getAll("method").map(String);
  const amounts = form.getAll("amount").map(String);

  const payments: { method: string; amount_cents: number }[] = [];
  for (let index = 0; index < methods.length; index += 1) {
    const raw = (amounts[index] ?? "").trim();
    if (raw === "") continue;
    const single = new FormData();
    single.set("amount", raw);
    const cents = money(single, "amount");
    if (cents === null || cents <= 0) {
      return { ok: false, message: `Valor inválido no pagamento ${index + 1}. Use o formato 150,00.` };
    }
    payments.push({ method: methods[index] ?? "", amount_cents: cents });
  }
  if (payments.length === 0) return { ok: false, message: "Informe ao menos um pagamento." };

  try {
    await api(`/v1/folios/${text(form, "folio_id")}/payments`, {
      method: "POST",
      idempotencyKey: text(form, "idempotency_key"),
      body: { payments },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath(`/admin/reservations/${reservationId}`);
  return { ok: true, message: "Pagamento registrado." };
}

export async function postCharge(_previous: ActionState, form: FormData): Promise<ActionState> {
  const cents = money(form, "amount");
  if (cents === null) return { ok: false, message: "Valor inválido.", errors: { amount: "Use o formato 15,00" } };
  try {
    await api(`/v1/folios/${text(form, "folio_id")}/charges`, {
      method: "POST",
      body: { kind: text(form, "kind") || "CHARGE", description: text(form, "description"), amount_cents: cents },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath(`/admin/reservations/${text(form, "reservation_id")}`);
  return { ok: true, message: "Lançamento adicionado." };
}

export async function voidEntry(_previous: ActionState, form: FormData): Promise<ActionState> {
  try {
    await api(`/v1/folios/${text(form, "folio_id")}/entries/${text(form, "entry_id")}/void`, {
      method: "POST",
      body: { reason: text(form, "reason") },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath(`/admin/reservations/${text(form, "reservation_id")}`);
  return { ok: true };
}

// ---- Ficha de registro (FNRH) ----

export async function registerGuests(_previous: ActionState, form: FormData): Promise<ActionState> {
  const reservationId = text(form, "reservation_id");
  const count = integer(form, "guest_count", 1);

  // Os campos de cada hóspede chegam com um sufixo numérico: full_name_0,
  // full_name_1... Aqui eles são reagrupados em objetos.
  const field = (name: string, index: number) => text(form, `${name}_${index}`);
  const guests = Array.from({ length: count }, (_, index) => ({
    full_name: field("full_name", index),
    email: field("email", index),
    phone: field("phone", index),
    birth_date: field("birth_date", index),
    nationality: field("nationality", index),
    profession: field("profession", index),
    document_type: field("document_type", index),
    document_number: field("document_number", index),
    address: {
      line: field("address_line", index),
      city: field("address_city", index),
      state: field("address_state", index),
      postal_code: field("address_postal_code", index),
      country: field("address_country", index) || "BR",
      district: "",
    },
    primary: index === 0,
    purpose_of_trip: text(form, "purpose_of_trip"),
    transport: text(form, "transport"),
    origin: { city: text(form, "origin_city"), state: text(form, "origin_state"), country: "BR" },
    destination: { city: text(form, "destination_city"), state: text(form, "destination_state"), country: "BR" },
    vehicle_plate: text(form, "vehicle_plate"),
  }));

  try {
    await api(`/v1/registrations/${reservationId}`, {
      method: "PUT",
      body: {
        property_id: text(form, "property_id"),
        check_in: text(form, "check_in"),
        check_out: text(form, "check_out"),
        guests,
      },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath(`/admin/reservations/${reservationId}`);
  return { ok: true, message: "Ficha de registro salva. O check-in está liberado." };
}
