"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, Select, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import type { Room, RoomType } from "@/lib/types";

import { createBlock, setRates } from "./actions";

const WEEKDAYS = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];

export function RatesForm({ roomTypes, from, to }: { roomTypes: RoomType[]; from: string; to: string }) {
  const [state, action] = useActionState(setRates, initialState);
  const errors = state.errors ?? {};
  return (
    <form action={action} className="grid gap-4 sm:grid-cols-6 sm:items-end">
      <Field label="Tipo de acomodação" name="room_type_id" className="sm:col-span-2">
        <Select name="room_type_id">
          {roomTypes.map((type) => (
            <option key={type.id} value={type.id}>
              {type.name}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="De" name="from" error={errors.from}>
        <Input name="from" type="date" defaultValue={from} required />
      </Field>
      <Field label="Até (inclusive)" name="to" error={errors.to}>
        <Input name="to" type="date" defaultValue={to} required />
      </Field>
      <Field label="Diária (R$)" name="price" error={errors.price ?? errors.price_cents} hint="Em branco = não alterar">
        <Input name="price" inputMode="decimal" placeholder="0,00" />
      </Field>
      <Field label="Estadia mínima" name="min_stay" error={errors.min_stay} hint="Noites; em branco = não alterar">
        <Input name="min_stay" type="number" min={1} max={30} />
      </Field>
      <fieldset className="sm:col-span-4">
        <legend className="mb-1 text-sm font-medium text-ink-700">Dias da semana (nenhum marcado = todos)</legend>
        <div className="flex flex-wrap gap-2">
          {WEEKDAYS.map((name, index) => (
            <label key={name} className="flex items-center gap-1.5 rounded-lg border border-ink-200 px-2.5 py-1.5 text-sm">
              <input type="checkbox" name="weekday" value={index} className="size-4 rounded border-ink-300" />
              {name}
            </label>
          ))}
        </div>
      </fieldset>
      <Field label="Vendas" name="closed" className="sm:col-span-2">
        <Select name="closed" defaultValue="">
          <option value="">Não alterar</option>
          <option value="false">Abrir vendas</option>
          <option value="true">Fechar vendas (stop-sell)</option>
        </Select>
      </Field>
      <div className="space-y-3 sm:col-span-6">
        <FormMessage state={state} />
        <SubmitButton>Aplicar ao período</SubmitButton>
      </div>
    </form>
  );
}

export function BlockForm({ rooms, from, to }: { rooms: Room[]; from: string; to: string }) {
  const [state, action] = useActionState(createBlock, initialState);
  const errors = state.errors ?? {};
  return (
    <form action={action} className="grid gap-4 sm:grid-cols-6 sm:items-end">
      <Field label="Quarto" name="room_id" className="sm:col-span-2">
        <Select name="room_id">
          {rooms.map((room) => (
            <option key={room.id} value={room.id}>
              {room.name}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="De" name="block_from" error={errors.check_in}>
        <Input id="block_from" name="from" type="date" defaultValue={from} required />
      </Field>
      <Field label="Até (inclusive)" name="block_to" error={errors.check_out}>
        <Input id="block_to" name="to" type="date" defaultValue={to} required />
      </Field>
      <Field label="Motivo" name="reason" className="sm:col-span-2">
        <Input name="reason" placeholder="Ex.: manutenção" />
      </Field>
      <div className="space-y-3 sm:col-span-6">
        <FormMessage state={state} />
        <SubmitButton variant="secondary">Bloquear quarto</SubmitButton>
      </div>
    </form>
  );
}
