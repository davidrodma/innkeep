"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, Select, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import type { Reservation, Room } from "@/lib/types";

import { cancelReservation, changeStay, transition } from "./actions";

/** Botão que dispara um comando simples (confirmar, check-in, check-out...). */
function CommandButton({
  id,
  command,
  children,
  variant = "primary",
}: {
  id: string;
  command: string;
  children: React.ReactNode;
  variant?: "primary" | "secondary";
}) {
  const [state, action] = useActionState(transition, initialState);
  return (
    <form action={action} className="space-y-2">
      <input type="hidden" name="id" value={id} />
      <input type="hidden" name="command" value={command} />
      <SubmitButton variant={variant} pendingText="Aguarde…">
        {children}
      </SubmitButton>
      {state.ok === false ? <FormMessage state={state} /> : null}
    </form>
  );
}

/**
 * Ações disponíveis conforme a situação da reserva. A tela só OFERECE o que
 * faz sentido; quem decide se a transição é válida é o agregado, no back-end
 * (se alguém forçar a chamada, a API responde com o erro da regra de negócio).
 */
export function ReservationActions({ reservation, rooms }: { reservation: Reservation; rooms: Room[] }) {
  const { id, status } = reservation;
  const canCancel = status === "REQUESTED" || status === "PENDING" || status === "CONFIRMED";
  const canChange = status === "PENDING" || status === "CONFIRMED" || status === "CHECKED_IN";

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start gap-3">
        {status === "PENDING" ? (
          <CommandButton id={id} command="confirm">
            Confirmar sem sinal
          </CommandButton>
        ) : null}
        {status === "CONFIRMED" ? (
          <CommandButton id={id} command="check-in">
            Fazer check-in
          </CommandButton>
        ) : null}
        {status === "CHECKED_IN" ? (
          <CommandButton id={id} command="check-out">
            Fazer check-out
          </CommandButton>
        ) : null}
        {status === "CONFIRMED" || status === "PENDING" ? (
          <CommandButton id={id} command="no-show" variant="secondary">
            Marcar não comparecimento
          </CommandButton>
        ) : null}
      </div>

      {status === "CONFIRMED" && !reservation.registration_completed ? (
        <p className="rounded-lg bg-warning-soft px-3 py-2 text-sm text-warning-ink">
          Preencha a ficha de registro do hóspede (FNRH), abaixo, para liberar o check-in.
        </p>
      ) : null}

      {canChange ? <ChangeStayForm reservation={reservation} rooms={rooms} /> : null}
      {canCancel ? <CancelForm id={id} /> : null}
    </div>
  );
}

function ChangeStayForm({ reservation, rooms }: { reservation: Reservation; rooms: Room[] }) {
  const [state, action] = useActionState(changeStay, initialState);
  const candidates = rooms.filter((room) => room.active);
  return (
    <details className="rounded-lg border border-ink-200">
      <summary className="cursor-pointer px-4 py-2.5 text-sm font-medium text-ink-800">Alterar datas ou quarto</summary>
      <form action={action} className="grid gap-4 border-t border-ink-200 p-4 sm:grid-cols-2">
        <input type="hidden" name="id" value={reservation.id} />
        <Field label="Entrada" name="check_in" error={state.errors?.check_in}>
          <Input name="check_in" type="date" defaultValue={reservation.check_in} required />
        </Field>
        <Field label="Saída" name="check_out" error={state.errors?.check_out}>
          <Input name="check_out" type="date" defaultValue={reservation.check_out} required />
        </Field>
        <Field label="Quarto" name="room_id">
          <Select name="room_id" defaultValue="">
            <option value="">Manter o atual, se estiver livre</option>
            {candidates.map((room) => (
              <option key={room.id} value={room.id}>
                {room.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Novo valor total (opcional)" name="quoted_total" hint="Em branco, recalcula pela tarifa.">
          <Input name="quoted_total" inputMode="decimal" placeholder="0,00" />
        </Field>
        <div className="space-y-3 sm:col-span-2">
          <FormMessage state={state} />
          <SubmitButton variant="secondary" pendingText="Solicitando…">
            Solicitar alteração
          </SubmitButton>
        </div>
      </form>
    </details>
  );
}

function CancelForm({ id }: { id: string }) {
  const [state, action] = useActionState(cancelReservation, initialState);
  return (
    <details className="rounded-lg border border-ink-200">
      <summary className="cursor-pointer px-4 py-2.5 text-sm font-medium text-danger-ink">Cancelar reserva</summary>
      <form action={action} className="space-y-3 border-t border-ink-200 p-4">
        <input type="hidden" name="id" value={id} />
        <Field label="Motivo do cancelamento" name="reason">
          <Input name="reason" placeholder="Ex.: pedido do hóspede" />
        </Field>
        <FormMessage state={state} />
        <SubmitButton variant="danger" pendingText="Cancelando…">
          Confirmar cancelamento
        </SubmitButton>
      </form>
    </details>
  );
}
