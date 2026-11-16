"use server";

import { revalidatePath } from "next/cache";

import { text } from "@/lib/action";
import { api } from "@/lib/api";

export async function anonymizeGuest(form: FormData) {
  const id = text(form, "id");
  await api(`/v1/guests/${id}/anonymize`, { method: "POST" });
  revalidatePath(`/admin/guests/${id}`);
}
