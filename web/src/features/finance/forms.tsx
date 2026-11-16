"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, Select, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import { EXPENSE_CATEGORY_LABELS, METHOD_LABELS } from "@/lib/labels";
import type { FeeRule } from "@/lib/types";

import { addCashMovement, createExpense, payExpense, saveFeeRules } from "./actions";

const PAYABLE_METHODS = ["PIX", "CASH", "DEBIT_CARD", "CREDIT_CARD", "BANK_TRANSFER", "OTHER"];

export function ExpenseForm({ propertyId, today }: { propertyId: string; today: string }) {
  const [state, action] = useActionState(createExpense, initialState);
  const errors = state.errors ?? {};
  return (
    <form action={action} className="grid gap-3 sm:grid-cols-6 sm:items-end">
      <input type="hidden" name="property_id" value={propertyId} />
      <Field label="Descrição" name="description" error={errors.description} className="sm:col-span-2">
        <Input name="description" placeholder="Ex.: conta de luz" required />
      </Field>
      <Field label="Categoria" name="category" className="sm:col-span-2">
        <Select name="category" defaultValue="OTHER">
          {Object.entries(EXPENSE_CATEGORY_LABELS).map(([code, text]) => (
            <option key={code} value={code}>
              {text}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Valor (R$)" name="amount" error={errors.amount ?? errors.amount_cents}>
        <Input name="amount" inputMode="decimal" placeholder="0,00" required />
      </Field>
      <Field label="Data / vencimento" name="due_date" error={errors.due_date}>
        <Input name="due_date" type="date" defaultValue={today} required />
      </Field>
      <Field label="Fornecedor" name="supplier" className="sm:col-span-2">
        <Input name="supplier" />
      </Field>
      <Field label="Já foi paga?" name="paid_method" hint="Pagamento em dinheiro sai do caixa." className="sm:col-span-2">
        <Select name="paid_method" defaultValue="">
          <option value="">Não, está a pagar</option>
          {PAYABLE_METHODS.map((method) => (
            <option key={method} value={method}>
              Sim, em {METHOD_LABELS[method]?.toLowerCase()}
            </option>
          ))}
        </Select>
      </Field>
      <SubmitButton className="sm:col-span-2">Registrar despesa</SubmitButton>
      {state.message ? (
        <div className="sm:col-span-6">
          <FormMessage state={state} />
        </div>
      ) : null}
    </form>
  );
}

export function PayExpenseForm({ id, today }: { id: string; today: string }) {
  const [state, action] = useActionState(payExpense, initialState);
  return (
    <form action={action} className="flex flex-wrap items-center justify-end gap-2">
      <input type="hidden" name="id" value={id} />
      <input type="hidden" name="paid_on" value={today} />
      <label htmlFor={`pay-${id}`} className="sr-only">
        Forma de pagamento
      </label>
      <Select id={`pay-${id}`} name="method" defaultValue="PIX" className="h-8! w-36 text-xs">
        {PAYABLE_METHODS.map((method) => (
          <option key={method} value={method}>
            {METHOD_LABELS[method]}
          </option>
        ))}
      </Select>
      <SubmitButton size="sm" variant="secondary" pendingText="…">
        Pagar
      </SubmitButton>
      {state.ok === false ? <span className="text-xs text-red-600">{state.message}</span> : null}
    </form>
  );
}

export function CashForm({ propertyId, today }: { propertyId: string; today: string }) {
  const [state, action] = useActionState(addCashMovement, initialState);
  const errors = state.errors ?? {};
  return (
    <form action={action} className="grid gap-3 sm:grid-cols-6 sm:items-end">
      <input type="hidden" name="property_id" value={propertyId} />
      <Field label="Tipo" name="kind" className="sm:col-span-2">
        <Select name="kind" defaultValue="OUT">
          <option value="OUT">Sangria (retirada)</option>
          <option value="IN">Suprimento (reforço)</option>
        </Select>
      </Field>
      <Field label="Valor (R$)" name="amount" error={errors.amount ?? errors.amount_cents}>
        <Input name="amount" inputMode="decimal" placeholder="0,00" required />
      </Field>
      <Field label="Data" name="occurred_on" error={errors.occurred_on}>
        <Input name="occurred_on" type="date" defaultValue={today} required />
      </Field>
      <Field label="Motivo" name="reason" error={errors.reason} className="sm:col-span-2">
        <Input name="reason" placeholder="Ex.: depósito no banco" required />
      </Field>
      <SubmitButton variant="secondary" className="sm:col-span-2">
        Registrar movimento
      </SubmitButton>
      {state.message ? (
        <div className="sm:col-span-6">
          <FormMessage state={state} />
        </div>
      ) : null}
    </form>
  );
}

export function FeeRulesForm({ propertyId, rules }: { propertyId: string; rules: FeeRule[] }) {
  const [state, action] = useActionState(saveFeeRules, initialState);
  return (
    <form action={action} className="space-y-4">
      <input type="hidden" name="property_id" value={propertyId} />
      <input type="hidden" name="count" value={rules.length} />
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {rules.map((rule) => (
          <div key={rule.method} className="flex items-center justify-between gap-3 rounded-lg border border-ink-200 px-3 py-2">
            <input type="hidden" name="method" value={rule.method} />
            <label htmlFor={`fee-${rule.method}`} className="text-sm text-ink-700">
              {METHOD_LABELS[rule.method] ?? rule.method}
            </label>
            <div className="flex items-center gap-1">
              <Input
                id={`fee-${rule.method}`}
                name="percent"
                inputMode="decimal"
                defaultValue={(rule.bps / 100).toString().replace(".", ",")}
                className="h-8! w-20 text-right"
              />
              <span className="text-sm text-ink-500">%</span>
            </div>
          </div>
        ))}
      </div>
      <FormMessage state={state} />
      <SubmitButton variant="secondary">Salvar taxas</SubmitButton>
    </form>
  );
}
