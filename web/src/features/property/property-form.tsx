"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, Select, SubmitButton, Textarea } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import type { Property } from "@/lib/types";

import { createProperty, updateProperty } from "./actions";

const KINDS: Record<string, string> = {
  POUSADA: "Pousada",
  HOTEL: "Hotel",
  HOSTEL: "Hostel",
  INN: "Hospedaria",
  APART_HOTEL: "Apart-hotel",
  VACATION_RENTAL: "Casa de temporada",
  OTHER: "Outro",
};

export function PropertyForm({ property }: { property?: Property }) {
  const [state, action] = useActionState(property ? updateProperty : createProperty, initialState);
  const errors = state.errors ?? {};
  const p = property;

  return (
    <form action={action} className="space-y-8">
      {p ? <input type="hidden" name="id" value={p.id} /> : null}

      <fieldset className="grid gap-4 sm:grid-cols-6">
        <legend className="mb-3 text-sm font-semibold text-ink-900">Identificação</legend>
        <Field label="Nome" name="name" error={errors.name} className="sm:col-span-4">
          <Input name="name" defaultValue={p?.name} required />
        </Field>
        <Field label="Tipo" name="kind" error={errors.kind} className="sm:col-span-2">
          <Select name="kind" defaultValue={p?.kind ?? "POUSADA"}>
            {Object.entries(KINDS).map(([code, text]) => (
              <option key={code} value={code}>
                {text}
              </option>
            ))}
          </Select>
        </Field>
        <Field
          label="Endereço no site (slug)"
          name="slug"
          error={errors.slug}
          hint="Parte da URL pública. Em branco, é gerado a partir do nome."
          className="sm:col-span-3"
        >
          <Input name="slug" defaultValue={p?.slug} placeholder="pousada-exemplo" />
        </Field>
        <Field label="Razão social" name="legal_name" className="sm:col-span-3">
          <Input name="legal_name" defaultValue={p?.legal_name} />
        </Field>
        <Field label="CNPJ / CPF" name="tax_id" className="sm:col-span-3">
          <Input name="tax_id" defaultValue={p?.tax_id} />
        </Field>
        <Field label="Cadastur" name="cadastur" hint="Cadastro no Ministério do Turismo." className="sm:col-span-3">
          <Input name="cadastur" defaultValue={p?.cadastur} />
        </Field>
        <Field label="Descrição (aparece no site)" name="description" className="sm:col-span-6">
          <Textarea name="description" defaultValue={p?.description} rows={3} />
        </Field>
      </fieldset>

      <fieldset className="grid gap-4 sm:grid-cols-6">
        <legend className="mb-3 text-sm font-semibold text-ink-900">Contato e endereço</legend>
        <Field label="E-mail" name="email" className="sm:col-span-3">
          <Input name="email" type="email" defaultValue={p?.email} />
        </Field>
        <Field label="Telefone / WhatsApp" name="phone" className="sm:col-span-3">
          <Input name="phone" type="tel" defaultValue={p?.phone} />
        </Field>
        <Field label="Logradouro e número" name="address_line" className="sm:col-span-4">
          <Input name="address_line" defaultValue={p?.address.line} />
        </Field>
        <Field label="Bairro" name="address_district" className="sm:col-span-2">
          <Input name="address_district" defaultValue={p?.address.district} />
        </Field>
        <Field label="Cidade" name="address_city" className="sm:col-span-3">
          <Input name="address_city" defaultValue={p?.address.city} />
        </Field>
        <Field label="UF" name="address_state" className="sm:col-span-1">
          <Input name="address_state" defaultValue={p?.address.state} maxLength={2} />
        </Field>
        <Field label="CEP" name="address_postal_code" className="sm:col-span-2">
          <Input name="address_postal_code" defaultValue={p?.address.postal_code} />
        </Field>
      </fieldset>

      <fieldset className="grid gap-4 sm:grid-cols-6">
        <legend className="mb-3 text-sm font-semibold text-ink-900">Políticas</legend>
        <Field label="Check-in a partir de" name="check_in_time" error={errors.check_in_time} className="sm:col-span-2">
          <Input name="check_in_time" type="time" defaultValue={p?.check_in_time ?? "14:00"} />
        </Field>
        <Field label="Check-out até" name="check_out_time" error={errors.check_out_time} className="sm:col-span-2">
          <Input name="check_out_time" type="time" defaultValue={p?.check_out_time ?? "12:00"} />
        </Field>
        <Field label="Fuso horário" name="timezone" error={errors.timezone} className="sm:col-span-2">
          <Select name="timezone" defaultValue={p?.timezone ?? "America/Sao_Paulo"}>
            {["America/Sao_Paulo", "America/Manaus", "America/Cuiaba", "America/Belem", "America/Fortaleza", "America/Recife", "America/Noronha", "America/Rio_Branco"].map(
              (zone) => (
                <option key={zone} value={zone}>
                  {zone.replace("America/", "").replace("_", " ")}
                </option>
              ),
            )}
          </Select>
        </Field>
        <Field
          label="Sinal para confirmar (%)"
          name="deposit_percent"
          error={errors.deposit_percent}
          hint="0 = reservas confirmadas sem sinal."
          className="sm:col-span-2"
        >
          <Input name="deposit_percent" type="number" min={0} max={100} defaultValue={p?.deposit_percent ?? 50} />
        </Field>
        <Field
          label="Prazo para pagar o sinal (horas)"
          name="hold_hours"
          error={errors.hold_minutes}
          hint="Depois disso a reserva expira. 0 = sem prazo."
          className="sm:col-span-2"
        >
          <Input name="hold_hours" type="number" min={0} max={168} defaultValue={p ? Math.round(p.hold_minutes / 60) : 24} />
        </Field>
        <Field label="Chave Pix" name="pix_key" hint="Mostrada ao hóspede para pagar o sinal." className="sm:col-span-2">
          <Input name="pix_key" defaultValue={p?.pix_key} />
        </Field>
        <Field label="Política de cancelamento" name="cancellation_policy" className="sm:col-span-6">
          <Textarea name="cancellation_policy" defaultValue={p?.cancellation_policy} rows={2} />
        </Field>
        <div className="flex items-center gap-2 sm:col-span-6">
          <input id="published" name="published" type="checkbox" defaultChecked={p?.published ?? false} className="size-4 rounded border-ink-300" />
          <label htmlFor="published" className="text-sm text-ink-700">
            Publicar no site de reservas
          </label>
        </div>
      </fieldset>

      <FormMessage state={state} />
      <SubmitButton>{p ? "Salvar alterações" : "Cadastrar propriedade"}</SubmitButton>
    </form>
  );
}
