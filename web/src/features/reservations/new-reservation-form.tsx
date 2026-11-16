"use client";

import { useActionState, useState } from "react";

import { Field, FormMessage, Input, Select, SubmitButton, Textarea } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import { formatMoney, nightsBetween } from "@/lib/format";
import type { Room, RoomType } from "@/lib/types";

import { createReservation } from "./actions";

type Props = {
  propertyId: string;
  roomTypes: RoomType[];
  rooms: Room[];
  defaults: { roomTypeId: string; roomId: string; checkIn: string; checkOut: string };
  idempotencyKey: string;
};

export function NewReservationForm({ propertyId, roomTypes, rooms, defaults, idempotencyKey }: Props) {
  const [state, action] = useActionState(createReservation, initialState);
  const [roomTypeId, setRoomTypeId] = useState(defaults.roomTypeId || roomTypes[0]?.id || "");
  const [checkIn, setCheckIn] = useState(defaults.checkIn);
  const [checkOut, setCheckOut] = useState(defaults.checkOut);

  const roomType = roomTypes.find((type) => type.id === roomTypeId);
  const typeRooms = rooms.filter((room) => room.room_type_id === roomTypeId && room.active);
  const nights = checkIn && checkOut ? nightsBetween(checkIn, checkOut) : 0;
  const errors = state.errors ?? {};

  return (
    <form action={action} className="space-y-6">
      <input type="hidden" name="property_id" value={propertyId} />
      <input type="hidden" name="idempotency_key" value={idempotencyKey} />

      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-3 text-sm font-semibold text-ink-900 sm:col-span-2">Estadia</legend>
        <Field label="Tipo de acomodação" name="room_type_id" error={errors.room_type_id}>
          <Select name="room_type_id" value={roomTypeId} onChange={(event) => setRoomTypeId(event.target.value)} required>
            {roomTypes.map((type) => (
              <option key={type.id} value={type.id}>
                {type.name} — {formatMoney(type.base_price_cents)}/noite
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Quarto" name="room_id" hint="Deixe em automático para o sistema escolher um quarto livre.">
          <Select name="room_id" defaultValue={defaults.roomId} key={roomTypeId}>
            <option value="">Automático</option>
            {typeRooms.map((room) => (
              <option key={room.id} value={room.id}>
                {room.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Entrada" name="check_in" error={errors.check_in}>
          <Input name="check_in" type="date" value={checkIn} onChange={(event) => setCheckIn(event.target.value)} required />
        </Field>
        <Field label="Saída" name="check_out" error={errors.check_out} hint={nights > 0 ? `${nights} noite(s)` : undefined}>
          <Input name="check_out" type="date" value={checkOut} min={checkIn} onChange={(event) => setCheckOut(event.target.value)} required />
        </Field>
        <Field label="Adultos" name="adults" error={errors.adults}>
          <Input name="adults" type="number" min={1} max={roomType?.max_occupancy ?? 50} defaultValue={2} required />
        </Field>
        <Field label="Crianças" name="children" error={errors.children}>
          <Input name="children" type="number" min={0} defaultValue={0} />
        </Field>
      </fieldset>

      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-3 text-sm font-semibold text-ink-900 sm:col-span-2">Titular da reserva</legend>
        <Field label="Nome completo" name="holder_name" error={errors["holder.name"]} className="sm:col-span-2">
          <Input name="holder_name" autoComplete="off" required />
        </Field>
        <Field label="E-mail" name="holder_email" error={errors["holder.email"]} hint="Recebe os avisos da reserva.">
          <Input name="holder_email" type="email" autoComplete="off" />
        </Field>
        <Field label="Telefone" name="holder_phone">
          <Input name="holder_phone" type="tel" autoComplete="off" />
        </Field>
      </fieldset>

      <fieldset className="grid gap-4 sm:grid-cols-2">
        <legend className="mb-3 text-sm font-semibold text-ink-900 sm:col-span-2">Valor e confirmação</legend>
        <Field
          label="Valor total negociado (opcional)"
          name="quoted_total"
          error={errors.quoted_total}
          hint={
            roomType && nights > 0
              ? `Em branco, vale a tarifa do período (referência: ${formatMoney(roomType.base_price_cents * nights)}).`
              : "Em branco, vale a tarifa do período."
          }
        >
          <Input name="quoted_total" inputMode="decimal" placeholder="0,00" />
        </Field>
        <div className="flex items-start gap-2 pt-7">
          <input id="confirm_now" name="confirm_now" type="checkbox" className="mt-0.5 size-4 rounded border-ink-300" />
          <label htmlFor="confirm_now" className="text-sm text-ink-700">
            Confirmar agora, sem exigir o sinal
          </label>
        </div>
        <Field label="Observações" name="notes" className="sm:col-span-2">
          <Textarea name="notes" />
        </Field>
      </fieldset>

      <FormMessage state={state} />
      <SubmitButton pendingText="Criando reserva…">Criar reserva</SubmitButton>
    </form>
  );
}
