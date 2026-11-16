"use server";

import { revalidatePath } from "next/cache";

import { checked, failure, text, type ActionState } from "@/lib/action";
import { api } from "@/lib/api";

export async function createUser(_previous: ActionState, form: FormData): Promise<ActionState> {
  try {
    await api("/v1/users", {
      method: "POST",
      body: {
        name: text(form, "name"),
        email: text(form, "email"),
        password: String(form.get("password") ?? ""),
        role: text(form, "role"),
        property_ids: form.getAll("property_ids").map(String),
      },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/users");
  return { ok: true, message: "Usuário criado. Entregue a senha inicial pessoalmente e peça que ele a troque." };
}

export async function updateUser(_previous: ActionState, form: FormData): Promise<ActionState> {
  try {
    await api(`/v1/users/${text(form, "id")}`, {
      method: "PUT",
      body: {
        name: text(form, "name"),
        role: text(form, "role"),
        property_ids: form.getAll("property_ids").map(String),
        active: checked(form, "active"),
      },
    });
    const password = String(form.get("new_password") ?? "");
    if (password) {
      await api(`/v1/users/${text(form, "id")}/password`, { method: "PUT", body: { new_password: password } });
    }
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/users");
  return { ok: true, message: "Usuário atualizado." };
}
