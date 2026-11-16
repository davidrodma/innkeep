"use server";

import { revalidatePath } from "next/cache";

import { checked, failure, text, type ActionState } from "@/lib/action";
import { api } from "@/lib/api";

export async function createConnection(_previous: ActionState, form: FormData): Promise<ActionState> {
  try {
    await api("/v1/channels/connections", {
      method: "POST",
      body: {
        property_id: text(form, "property_id"),
        channel: text(form, "channel"),
        mode: text(form, "mode"),
        name: text(form, "name"),
        external_property_id: text(form, "external_property_id"),
        client_id: text(form, "client_id"),
        client_secret: String(form.get("client_secret") ?? ""),
        base_url: text(form, "base_url"),
        active: true,
      },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/channels");
  return { ok: true, message: "Canal conectado. Agora mapeie os tipos de acomodação." };
}

export async function updateConnection(_previous: ActionState, form: FormData): Promise<ActionState> {
  try {
    await api(`/v1/channels/connections/${text(form, "id")}`, {
      method: "PUT",
      body: {
        name: text(form, "name"),
        external_property_id: text(form, "external_property_id"),
        client_id: text(form, "client_id"),
        client_secret: String(form.get("client_secret") ?? ""),
        base_url: text(form, "base_url"),
        active: checked(form, "active"),
      },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/channels");
  return { ok: true, message: "Conexão atualizada." };
}

export async function createMapping(_previous: ActionState, form: FormData): Promise<ActionState> {
  try {
    await api(`/v1/channels/connections/${text(form, "connection_id")}/mappings`, {
      method: "POST",
      body: {
        room_type_id: text(form, "room_type_id"),
        external_room_id: text(form, "external_room_id"),
        external_rate_id: text(form, "external_rate_id"),
        ical_import_url: text(form, "ical_import_url"),
      },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/channels");
  return { ok: true, message: "Mapeamento criado. A disponibilidade completa está sendo enviada ao canal." };
}

export async function deleteMapping(form: FormData) {
  await api(`/v1/channels/mappings/${text(form, "id")}`, { method: "DELETE" });
  revalidatePath("/admin/channels");
}

export async function deleteConnection(form: FormData) {
  await api(`/v1/channels/connections/${text(form, "id")}`, { method: "DELETE" });
  revalidatePath("/admin/channels");
}

export async function syncConnection(_previous: ActionState, form: FormData): Promise<ActionState> {
  const id = text(form, "id");
  try {
    if (text(form, "kind") === "pull") {
      const result = await api<{ changes: number }>(`/v1/channels/connections/${id}/pull`, { method: "POST" });
      return { ok: true, message: `Consulta feita: ${result.changes} novidade(s).` };
    }
    const result = await api<{ room_types: number }>(`/v1/channels/connections/${id}/sync`, { method: "POST" });
    revalidatePath("/admin/channels");
    return { ok: true, message: `Reenvio completo solicitado para ${result.room_types} tipo(s) de acomodação.` };
  } catch (error) {
    return failure(error);
  }
}
