"use server";

import { revalidatePath } from "next/cache";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { checked, failure, integer, money, text, type ActionState } from "@/lib/action";
import { api } from "@/lib/api";
import { PROPERTY_COOKIE, cookieOptions } from "@/lib/session-cookies";
import type { Property } from "@/lib/types";

function propertyBody(form: FormData) {
  return {
    slug: text(form, "slug"),
    name: text(form, "name"),
    legal_name: text(form, "legal_name"),
    tax_id: text(form, "tax_id"),
    cadastur: text(form, "cadastur"),
    kind: text(form, "kind"),
    description: text(form, "description"),
    email: text(form, "email"),
    phone: text(form, "phone"),
    address: {
      line: text(form, "address_line"),
      district: text(form, "address_district"),
      city: text(form, "address_city"),
      state: text(form, "address_state"),
      postal_code: text(form, "address_postal_code"),
      country: text(form, "address_country") || "BR",
    },
    timezone: text(form, "timezone"),
    currency: text(form, "currency") || "BRL",
    check_in_time: text(form, "check_in_time"),
    check_out_time: text(form, "check_out_time"),
    deposit_percent: integer(form, "deposit_percent", 0),
    hold_minutes: integer(form, "hold_hours", 0) * 60,
    cancellation_policy: text(form, "cancellation_policy"),
    pix_key: text(form, "pix_key"),
    published: checked(form, "published"),
  };
}

export async function createProperty(_previous: ActionState, form: FormData): Promise<ActionState> {
  let property: Property;
  try {
    property = await api<Property>("/v1/properties", { method: "POST", body: propertyBody(form) });
  } catch (error) {
    return failure(error);
  }
  const store = await cookies();
  store.set(PROPERTY_COOKIE, property.id, cookieOptions(new Date(Date.now() + 365 * 86_400_000)));
  redirect("/admin/property");
}

export async function updateProperty(_previous: ActionState, form: FormData): Promise<ActionState> {
  try {
    await api(`/v1/properties/${text(form, "id")}`, { method: "PUT", body: propertyBody(form) });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin", "layout");
  return { ok: true, message: "Dados da propriedade salvos." };
}

export async function saveRoomType(_previous: ActionState, form: FormData): Promise<ActionState> {
  const price = money(form, "base_price");
  if (price === null) return { ok: false, message: "Informe a diária.", errors: { base_price: "Use o formato 150,00" } };
  const body = {
    name: text(form, "name"),
    description: text(form, "description"),
    kind: text(form, "kind") || "PRIVATE",
    max_occupancy: integer(form, "max_occupancy", 2),
    bed_configuration: text(form, "bed_configuration"),
    base_price_cents: price,
    amenities: text(form, "amenities")
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean),
    photo_urls: text(form, "photo_urls")
      .split(/\s+/)
      .map((item) => item.trim())
      .filter(Boolean),
    active: checked(form, "active"),
    sort_order: integer(form, "sort_order", 0),
  };
  const id = text(form, "id");
  try {
    if (id) {
      await api(`/v1/room-types/${id}`, { method: "PUT", body });
    } else {
      await api(`/v1/properties/${text(form, "property_id")}/room-types`, { method: "POST", body });
    }
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/property");
  return { ok: true, message: id ? "Tipo de acomodação atualizado." : "Tipo de acomodação criado." };
}

export async function saveRoom(_previous: ActionState, form: FormData): Promise<ActionState> {
  const body = {
    room_type_id: text(form, "room_type_id"),
    name: text(form, "name"),
    floor: text(form, "floor"),
    notes: text(form, "notes"),
    active: checked(form, "active"),
  };
  const id = text(form, "id");
  try {
    if (id) {
      await api(`/v1/rooms/${id}`, { method: "PUT", body });
    } else {
      await api(`/v1/properties/${text(form, "property_id")}/rooms`, { method: "POST", body });
    }
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/property");
  return { ok: true, message: id ? "Quarto atualizado." : "Quarto criado." };
}

export async function setHousekeeping(form: FormData) {
  await api(`/v1/rooms/${text(form, "room_id")}/housekeeping`, {
    method: "PUT",
    body: { status: text(form, "status") },
  });
  revalidatePath("/admin/housekeeping");
}
