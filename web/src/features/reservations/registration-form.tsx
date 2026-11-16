"use client";

import { useActionState, useState } from "react";

import { Button } from "@/components/ui/button";
import { Field, FormMessage, Input, Select, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import { DOCUMENT_LABELS, TRANSPORT_LABELS, TRIP_PURPOSE_LABELS } from "@/lib/labels";
import type { Reservation } from "@/lib/types";

import { registerGuests } from "./actions";

/**
 * Ficha Nacional de Registro de Hóspedes (FNRH). O primeiro hóspede é o
 * titular; os demais são acompanhantes. Os dados de viagem (procedência,
 * destino, motivo, transporte) valem para o grupo.
 */
export function RegistrationForm({ reservation }: { reservation: Reservation }) {
  const [state, action] = useActionState(registerGuests, initialState);
  const expected = Math.max(reservation.adults + reservation.children, 1);
  const [count, setCount] = useState(1);
  const errors = state.errors ?? {};

  return (
    <form action={action} className="space-y-6">
      <input type="hidden" name="reservation_id" value={reservation.id} />
      <input type="hidden" name="property_id" value={reservation.property_id} />
      <input type="hidden" name="check_in" value={reservation.check_in} />
      <input type="hidden" name="check_out" value={reservation.check_out} />
      <input type="hidden" name="guest_count" value={count} />

      {Array.from({ length: count }, (_, index) => (
        <fieldset key={index} className="grid gap-4 rounded-lg border border-ink-200 p-4 sm:grid-cols-6">
          <legend className="px-1 text-sm font-semibold text-ink-900">
            {index === 0 ? "Hóspede titular" : `Acompanhante ${index}`}
          </legend>
          <Field label="Nome completo" name={`full_name_${index}`} error={index === 0 ? errors.full_name : undefined} className="sm:col-span-4">
            <Input name={`full_name_${index}`} defaultValue={index === 0 ? reservation.holder.name : ""} required />
          </Field>
          <Field label="Nascimento" name={`birth_date_${index}`} error={index === 0 ? errors.birth_date : undefined} className="sm:col-span-2">
            <Input name={`birth_date_${index}`} type="date" />
          </Field>
          <Field label="Documento" name={`document_type_${index}`} className="sm:col-span-2">
            <Select name={`document_type_${index}`} defaultValue="CPF">
              {Object.entries(DOCUMENT_LABELS).map(([code, text]) => (
                <option key={code} value={code}>
                  {text}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Número" name={`document_number_${index}`} error={index === 0 ? errors.document_number : undefined} className="sm:col-span-2">
            <Input name={`document_number_${index}`} autoComplete="off" required />
          </Field>
          <Field label="Nacionalidade" name={`nationality_${index}`} className="sm:col-span-2">
            <Input name={`nationality_${index}`} defaultValue="Brasileira" />
          </Field>
          {index === 0 ? (
            <>
              <Field label="E-mail" name="email_0" error={errors.email} className="sm:col-span-3">
                <Input name="email_0" type="email" defaultValue={reservation.holder.email ?? ""} />
              </Field>
              <Field label="Telefone" name="phone_0" className="sm:col-span-3">
                <Input name="phone_0" type="tel" defaultValue={reservation.holder.phone ?? ""} />
              </Field>
              <Field label="Profissão" name="profession_0" className="sm:col-span-2">
                <Input name="profession_0" />
              </Field>
              <Field label="Endereço" name="address_line_0" className="sm:col-span-4">
                <Input name="address_line_0" placeholder="Rua, número e bairro" />
              </Field>
              <Field label="Cidade" name="address_city_0" className="sm:col-span-3">
                <Input name="address_city_0" />
              </Field>
              <Field label="UF" name="address_state_0" className="sm:col-span-1">
                <Input name="address_state_0" maxLength={2} />
              </Field>
              <Field label="CEP" name="address_postal_code_0" className="sm:col-span-2">
                <Input name="address_postal_code_0" inputMode="numeric" />
              </Field>
            </>
          ) : null}
        </fieldset>
      ))}

      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" size="sm" onClick={() => setCount((current) => Math.min(current + 1, 20))}>
          Adicionar acompanhante
        </Button>
        {count > 1 ? (
          <Button variant="ghost" size="sm" onClick={() => setCount((current) => current - 1)}>
            Remover último
          </Button>
        ) : null}
        {count < expected ? (
          <p className="self-center text-xs text-ink-500">A reserva prevê {expected} hóspede(s).</p>
        ) : null}
      </div>

      <fieldset className="grid gap-4 sm:grid-cols-6">
        <legend className="mb-3 text-sm font-semibold text-ink-900">Dados da viagem</legend>
        <Field label="Motivo da viagem" name="purpose_of_trip" className="sm:col-span-3">
          <Select name="purpose_of_trip" defaultValue="LEISURE">
            {Object.entries(TRIP_PURPOSE_LABELS).map(([code, text]) => (
              <option key={code} value={code}>
                {text}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Meio de transporte" name="transport" className="sm:col-span-2">
          <Select name="transport" defaultValue="CAR">
            {Object.entries(TRANSPORT_LABELS).map(([code, text]) => (
              <option key={code} value={code}>
                {text}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Placa" name="vehicle_plate" className="sm:col-span-1">
          <Input name="vehicle_plate" />
        </Field>
        <Field label="Última procedência (cidade)" name="origin_city" className="sm:col-span-2">
          <Input name="origin_city" />
        </Field>
        <Field label="UF" name="origin_state" className="sm:col-span-1">
          <Input name="origin_state" maxLength={2} />
        </Field>
        <Field label="Próximo destino (cidade)" name="destination_city" className="sm:col-span-2">
          <Input name="destination_city" />
        </Field>
        <Field label="UF" name="destination_state" className="sm:col-span-1">
          <Input name="destination_state" maxLength={2} />
        </Field>
      </fieldset>

      <p className="text-xs text-ink-500">
        Os dados são coletados por exigência da Lei Geral do Turismo (Lei 11.771/2008) e tratados conforme a LGPD. O
        número do documento é armazenado criptografado.
      </p>
      <FormMessage state={state} />
      <SubmitButton pendingText="Salvando ficha…">Salvar ficha de registro</SubmitButton>
    </form>
  );
}
