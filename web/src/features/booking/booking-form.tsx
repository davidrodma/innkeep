"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, SubmitButton, Textarea } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";

import { bookStay } from "./actions";

type Props = {
  propertyId: string;
  roomTypeId: string;
  checkIn: string;
  checkOut: string;
  adults: number;
  /** Número de crianças ("children" é nome reservado de prop no React). */
  childCount: number;
  cancellationPolicy: string;
  idempotencyKey: string;
};

export function BookingForm(props: Props) {
  const [state, action] = useActionState(bookStay, initialState);
  const errors = state.errors ?? {};
  return (
    <form action={action} className="space-y-4">
      <input type="hidden" name="property_id" value={props.propertyId} />
      <input type="hidden" name="room_type_id" value={props.roomTypeId} />
      <input type="hidden" name="check_in" value={props.checkIn} />
      <input type="hidden" name="check_out" value={props.checkOut} />
      <input type="hidden" name="adults" value={props.adults} />
      <input type="hidden" name="children" value={props.childCount} />
      <input type="hidden" name="idempotency_key" value={props.idempotencyKey} />

      <Field label="Nome completo" name="name" error={errors["holder.name"]}>
        <Input name="name" autoComplete="name" required />
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="E-mail" name="email" error={errors["holder.email"]} hint="Enviaremos a confirmação para este e-mail.">
          <Input name="email" type="email" autoComplete="email" required />
        </Field>
        <Field label="Telefone / WhatsApp" name="phone">
          <Input name="phone" type="tel" autoComplete="tel" />
        </Field>
      </div>
      <Field label="Observações (opcional)" name="notes" hint="Horário previsto de chegada, pedidos especiais…">
        <Textarea name="notes" rows={2} />
      </Field>

      <div className="flex items-start gap-2">
        <input id="accept_terms" name="accept_terms" type="checkbox" className="mt-0.5 size-4 rounded border-ink-300" required />
        <label htmlFor="accept_terms" className="text-sm text-ink-700">
          Li e aceito a política de cancelamento{props.cancellationPolicy ? `: ${props.cancellationPolicy}` : "."} Autorizo o uso dos
          meus dados para a gestão desta reserva.
        </label>
      </div>

      {errors.check_in || errors.check_out ? (
        <p className="text-sm text-red-700">{errors.check_in ?? errors.check_out}</p>
      ) : null}
      <FormMessage state={state} />
      <SubmitButton pendingText="Enviando reserva…" className="w-full sm:w-auto">
        Confirmar reserva
      </SubmitButton>
    </form>
  );
}
