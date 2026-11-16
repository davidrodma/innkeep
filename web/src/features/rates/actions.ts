"use server";

import { revalidatePath } from "next/cache";

import { failure, integer, money, text, type ActionState } from "@/lib/action";
import { api } from "@/lib/api";

/**
 * Altera tarifas e restrições de um período. Cada alteração gera, no
 * inventário, um evento "tarifa alterada" — que o serviço de canais repassa
 * ao Booking.com automaticamente.
 */
export async function setRates(_previous: ActionState, form: FormData): Promise<ActionState> {
  const price = money(form, "price");
  if (text(form, "price") !== "" && price === null) {
    return { ok: false, message: "Preço inválido.", errors: { price: "Use o formato 150,00" } };
  }
  const minStay = text(form, "min_stay");
  const closed = text(form, "closed");
  try {
    await api("/v1/rates", {
      method: "PUT",
      body: {
        room_type_id: text(form, "room_type_id"),
        from: text(form, "from"),
        // A tela pede a data final INCLUSIVA (mais natural para pessoas); a
        // API trabalha com intervalo semiaberto [from, to). Daí o +1 dia.
        to: addOneDay(text(form, "to")),
        weekdays: form.getAll("weekday").map((day) => Number(day)),
        price_cents: price,
        min_stay: minStay === "" ? null : integer(form, "min_stay", 1),
        closed: closed === "" ? null : closed === "true",
      },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/rates");
  return { ok: true, message: "Tarifas atualizadas e enviadas aos canais conectados." };
}

export async function createBlock(_previous: ActionState, form: FormData): Promise<ActionState> {
  try {
    await api("/v1/blocks", {
      method: "POST",
      body: { room_id: text(form, "room_id"), from: text(form, "from"), to: addOneDay(text(form, "to")), reason: text(form, "reason") },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/rates");
  revalidatePath("/admin/calendar");
  return { ok: true, message: "Quarto bloqueado no período." };
}

export async function deleteBlock(form: FormData) {
  await api(`/v1/blocks/${text(form, "id")}`, { method: "DELETE" });
  revalidatePath("/admin/rates");
  revalidatePath("/admin/calendar");
}

function addOneDay(iso: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(iso)) return iso;
  const date = new Date(`${iso}T00:00:00Z`);
  date.setUTCDate(date.getUTCDate() + 1);
  return date.toISOString().slice(0, 10);
}
