"use client";

import { Plus, Trash2 } from "lucide-react";
import { useActionState, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Field, FormMessage, Input, Select, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import { formatMoney, parseMoney } from "@/lib/format";
import { METHOD_LABELS } from "@/lib/labels";

import { postCharge, recordPayments, voidEntry } from "./actions";

type Row = { key: number; method: string; amount: string };

/**
 * Lançamento de pagamentos com VÁRIAS formas na mesma operação — o hóspede
 * paga parte em Pix, parte em dinheiro, parte no cartão. Todas as linhas vão
 * em uma única requisição, e o back-end grava todas ou nenhuma.
 */
export function PaymentForm({
  folioId,
  reservationId,
  balanceCents,
  depositPendingCents,
  idempotencyKey,
}: {
  folioId: string;
  reservationId: string;
  balanceCents: number;
  depositPendingCents: number;
  idempotencyKey: string;
}) {
  const [state, action] = useActionState(recordPayments, initialState);
  const [rows, setRows] = useState<Row[]>([{ key: 0, method: "PIX", amount: "" }]);
  // Cada linha precisa de uma chave única e estável para o React. Um contador
  // em useRef garante isso sem provocar novas renderizações.
  const lastKey = useRef(0);
  const newKey = () => ++lastKey.current;

  const total = rows.reduce((sum, row) => sum + (parseMoney(row.amount) ?? 0), 0);
  const remaining = balanceCents - total;

  const update = (key: number, patch: Partial<Row>) =>
    setRows((current) => current.map((row) => (row.key === key ? { ...row, ...patch } : row)));
  const fill = (cents: number) =>
    setRows([{ key: newKey(), method: rows[0]?.method ?? "PIX", amount: (cents / 100).toFixed(2).replace(".", ",") }]);

  return (
    <form action={action} className="space-y-3">
      <input type="hidden" name="folio_id" value={folioId} />
      <input type="hidden" name="reservation_id" value={reservationId} />
      {/* A chave muda a cada lançamento concluído (o servidor gera outra). */}
      <input type="hidden" name="idempotency_key" value={idempotencyKey} />

      {rows.map((row, index) => (
        <div key={row.key} className="flex items-end gap-2">
          <div className="flex-1">
            <label htmlFor={`method-${row.key}`} className={index === 0 ? "mb-1 block text-sm font-medium text-ink-700" : "sr-only"}>
              Forma de pagamento
            </label>
            <Select id={`method-${row.key}`} name="method" value={row.method} onChange={(event) => update(row.key, { method: event.target.value })}>
              {Object.entries(METHOD_LABELS).map(([code, text]) => (
                <option key={code} value={code}>
                  {text}
                </option>
              ))}
            </Select>
          </div>
          <div className="w-32">
            <label htmlFor={`amount-${row.key}`} className={index === 0 ? "mb-1 block text-sm font-medium text-ink-700" : "sr-only"}>
              Valor (R$)
            </label>
            <Input
              id={`amount-${row.key}`}
              name="amount"
              inputMode="decimal"
              placeholder="0,00"
              value={row.amount}
              onChange={(event) => update(row.key, { amount: event.target.value })}
            />
          </div>
          <Button
            variant="ghost"
            aria-label="Remover esta forma de pagamento"
            disabled={rows.length === 1}
            onClick={() => setRows((current) => current.filter((item) => item.key !== row.key))}
            className="px-2"
          >
            <Trash2 className="size-4" aria-hidden />
          </Button>
        </div>
      ))}

      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => setRows((current) => [...current, { key: newKey(), method: "CASH", amount: "" }])}
        >
          <Plus className="size-4" aria-hidden />
          Outra forma de pagamento
        </Button>
        {depositPendingCents > 0 ? (
          <Button variant="ghost" size="sm" onClick={() => fill(depositPendingCents)}>
            Preencher sinal ({formatMoney(depositPendingCents)})
          </Button>
        ) : null}
        {balanceCents > 0 ? (
          <Button variant="ghost" size="sm" onClick={() => fill(balanceCents)}>
            Preencher saldo ({formatMoney(balanceCents)})
          </Button>
        ) : null}
      </div>

      <p className="tabular text-sm text-ink-600">
        Total deste lançamento: <strong className="text-ink-900">{formatMoney(total)}</strong>
        {total > 0 ? (
          <>
            {" · "}
            {remaining > 0 ? `Ficará faltando ${formatMoney(remaining)}` : remaining < 0 ? `Crédito de ${formatMoney(-remaining)}` : "Conta quitada"}
          </>
        ) : null}
      </p>

      <FormMessage state={state} />
      <SubmitButton pendingText="Registrando…">Registrar pagamento</SubmitButton>
    </form>
  );
}

/** Lançamento de consumo extra ou desconto. */
export function ChargeForm({ folioId, reservationId }: { folioId: string; reservationId: string }) {
  const [state, action] = useActionState(postCharge, initialState);
  return (
    <form action={action} className="grid gap-3 sm:grid-cols-[9rem_1fr_8rem_auto] sm:items-end">
      <input type="hidden" name="folio_id" value={folioId} />
      <input type="hidden" name="reservation_id" value={reservationId} />
      <Field label="Tipo" name="kind">
        <Select name="kind" defaultValue="CHARGE">
          <option value="CHARGE">Consumo / extra</option>
          <option value="DISCOUNT">Desconto</option>
        </Select>
      </Field>
      <Field label="Descrição" name="description" error={state.errors?.description}>
        <Input name="description" placeholder="Ex.: frigobar, lavanderia" required />
      </Field>
      <Field label="Valor (R$)" name="amount" error={state.errors?.amount}>
        <Input name="amount" inputMode="decimal" placeholder="0,00" required />
      </Field>
      <SubmitButton variant="secondary" pendingText="Lançando…">
        Lançar
      </SubmitButton>
      {state.message ? (
        <div className="sm:col-span-4">
          <FormMessage state={state} />
        </div>
      ) : null}
    </form>
  );
}

/** Anulação de um lançamento (o lançamento continua visível, riscado). */
export function VoidEntryForm({ folioId, reservationId, entryId }: { folioId: string; reservationId: string; entryId: string }) {
  const [state, action] = useActionState(voidEntry, initialState);
  return (
    <details className="relative">
      <summary className="cursor-pointer text-xs text-ink-500 hover:text-red-700">Anular</summary>
      <form action={action} className="absolute right-0 z-20 mt-1 w-64 space-y-2 rounded-lg border border-ink-200 bg-white p-3 shadow-lg">
        <input type="hidden" name="folio_id" value={folioId} />
        <input type="hidden" name="reservation_id" value={reservationId} />
        <input type="hidden" name="entry_id" value={entryId} />
        <label htmlFor={`void-${entryId}`} className="block text-xs font-medium text-ink-700">
          Motivo da anulação
        </label>
        <Input id={`void-${entryId}`} name="reason" required />
        <FormMessage state={state} />
        <SubmitButton variant="danger" size="sm" pendingText="Anulando…">
          Anular lançamento
        </SubmitButton>
      </form>
    </details>
  );
}
