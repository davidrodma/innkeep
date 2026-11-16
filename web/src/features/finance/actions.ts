"use server";

import { revalidatePath } from "next/cache";

import { failure, integer, money, text, type ActionState } from "@/lib/action";
import { api } from "@/lib/api";

export async function createExpense(_previous: ActionState, form: FormData): Promise<ActionState> {
  const amount = money(form, "amount");
  if (amount === null) return { ok: false, message: "Valor inválido.", errors: { amount: "Use o formato 150,00" } };
  try {
    const expense = await api<{ id: string }>("/v1/expenses", {
      method: "POST",
      body: {
        property_id: text(form, "property_id"),
        category: text(form, "category"),
        description: text(form, "description"),
        supplier: text(form, "supplier"),
        amount_cents: amount,
        due_date: text(form, "due_date"),
      },
    });
    // Despesa já paga no ato: registra o pagamento em seguida.
    if (text(form, "paid_method")) {
      await api(`/v1/expenses/${expense.id}/pay`, {
        method: "POST",
        body: { method: text(form, "paid_method"), paid_on: text(form, "due_date") },
      });
    }
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/finance", "layout");
  return { ok: true, message: "Despesa registrada." };
}

export async function payExpense(_previous: ActionState, form: FormData): Promise<ActionState> {
  try {
    await api(`/v1/expenses/${text(form, "id")}/pay`, {
      method: "POST",
      body: { method: text(form, "method"), paid_on: text(form, "paid_on") },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/finance", "layout");
  return { ok: true };
}

export async function deleteExpense(form: FormData) {
  await api(`/v1/expenses/${text(form, "id")}`, { method: "DELETE" });
  revalidatePath("/admin/finance", "layout");
}

export async function addCashMovement(_previous: ActionState, form: FormData): Promise<ActionState> {
  const amount = money(form, "amount");
  if (amount === null) return { ok: false, message: "Valor inválido.", errors: { amount: "Use o formato 150,00" } };
  try {
    await api("/v1/cash/movements", {
      method: "POST",
      body: {
        property_id: text(form, "property_id"),
        kind: text(form, "kind"),
        amount_cents: amount,
        reason: text(form, "reason"),
        occurred_on: text(form, "occurred_on"),
      },
    });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/finance", "layout");
  return { ok: true, message: "Movimento de caixa registrado." };
}

export async function saveFeeRules(_previous: ActionState, form: FormData): Promise<ActionState> {
  const rules = form.getAll("method").map((method, index) => {
    const percent = String(form.getAll("percent")[index] ?? "0").replace(",", ".");
    return { method: String(method), bps: Math.round(Number.parseFloat(percent || "0") * 100) };
  });
  if (rules.some((rule) => !Number.isFinite(rule.bps))) {
    return { ok: false, message: "Use números para as taxas, por exemplo 15 ou 4,99." };
  }
  try {
    await api("/v1/fee-rules", { method: "PUT", body: { property_id: text(form, "property_id"), rules } });
  } catch (error) {
    return failure(error);
  }
  revalidatePath("/admin/finance", "layout");
  return { ok: true, message: `Taxas salvas (${integer(form, "count", rules.length)} formas de pagamento).` };
}
