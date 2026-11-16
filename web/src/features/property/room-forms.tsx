"use client";

import { useActionState } from "react";

import { Field, FormMessage, Input, Select, SubmitButton, Textarea } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import type { Room, RoomType } from "@/lib/types";

import { saveRoom, saveRoomType } from "./actions";

export function RoomTypeForm({ propertyId, roomType }: { propertyId: string; roomType?: RoomType }) {
  const [state, action] = useActionState(saveRoomType, initialState);
  const errors = state.errors ?? {};
  const t = roomType;
  const prefix = t?.id ?? "new";

  return (
    <form action={action} className="grid gap-4 sm:grid-cols-6">
      <input type="hidden" name="property_id" value={propertyId} />
      {t ? <input type="hidden" name="id" value={t.id} /> : null}
      <Field label="Nome" name={`${prefix}-name`} error={errors.name} className="sm:col-span-3">
        <Input id={`${prefix}-name`} name="name" defaultValue={t?.name} placeholder="Ex.: Suíte com varanda" required />
      </Field>
      <Field label="Modalidade" name={`${prefix}-kind`} className="sm:col-span-3">
        <Select id={`${prefix}-kind`} name="kind" defaultValue={t?.kind ?? "PRIVATE"}>
          <option value="PRIVATE">Privativo (vende o quarto inteiro)</option>
          <option value="SHARED">Compartilhado (cadastre cada cama como um quarto)</option>
        </Select>
      </Field>
      <Field label="Diária padrão (R$)" name={`${prefix}-base_price`} error={errors.base_price ?? errors.base_price_cents} className="sm:col-span-2">
        <Input
          id={`${prefix}-base_price`}
          name="base_price"
          inputMode="decimal"
          defaultValue={t ? (t.base_price_cents / 100).toFixed(2).replace(".", ",") : ""}
          required
        />
      </Field>
      <Field label="Capacidade" name={`${prefix}-max_occupancy`} error={errors.max_occupancy} className="sm:col-span-2">
        <Input id={`${prefix}-max_occupancy`} name="max_occupancy" type="number" min={1} max={50} defaultValue={t?.max_occupancy ?? 2} />
      </Field>
      <Field label="Ordem de exibição" name={`${prefix}-sort_order`} className="sm:col-span-2">
        <Input id={`${prefix}-sort_order`} name="sort_order" type="number" defaultValue={t?.sort_order ?? 0} />
      </Field>
      <Field label="Camas" name={`${prefix}-bed_configuration`} className="sm:col-span-3">
        <Input id={`${prefix}-bed_configuration`} name="bed_configuration" defaultValue={t?.bed_configuration} placeholder="1 cama de casal" />
      </Field>
      <Field label="Comodidades (separe por vírgula)" name={`${prefix}-amenities`} className="sm:col-span-3">
        <Input id={`${prefix}-amenities`} name="amenities" defaultValue={t?.amenities.join(", ")} placeholder="Wi-Fi, ar-condicionado" />
      </Field>
      <Field label="Descrição" name={`${prefix}-description`} className="sm:col-span-6">
        <Textarea id={`${prefix}-description`} name="description" defaultValue={t?.description} rows={2} />
      </Field>
      <Field label="Fotos (URLs, uma por linha)" name={`${prefix}-photo_urls`} error={errors.photo_urls} className="sm:col-span-6">
        <Textarea id={`${prefix}-photo_urls`} name="photo_urls" defaultValue={t?.photo_urls.join("\n")} rows={2} />
      </Field>
      <div className="flex items-center gap-2 sm:col-span-6">
        <input id={`${prefix}-active`} name="active" type="checkbox" defaultChecked={t?.active ?? true} className="size-4 rounded border-ink-300" />
        <label htmlFor={`${prefix}-active`} className="text-sm text-ink-700">
          Ativo (disponível para venda)
        </label>
      </div>
      <div className="space-y-3 sm:col-span-6">
        <FormMessage state={state} />
        <SubmitButton variant={t ? "secondary" : "primary"}>{t ? "Salvar tipo" : "Adicionar tipo de acomodação"}</SubmitButton>
      </div>
    </form>
  );
}

export function RoomForm({ propertyId, roomTypes, room }: { propertyId: string; roomTypes: RoomType[]; room?: Room }) {
  const [state, action] = useActionState(saveRoom, initialState);
  const errors = state.errors ?? {};
  const prefix = room?.id ?? "new-room";

  return (
    <form action={action} className="grid gap-3 sm:grid-cols-[1fr_1.5fr_1fr_auto_auto] sm:items-end">
      <input type="hidden" name="property_id" value={propertyId} />
      {room ? <input type="hidden" name="id" value={room.id} /> : null}
      <Field label="Nome / número" name={`${prefix}-name`} error={errors.name}>
        <Input id={`${prefix}-name`} name="name" defaultValue={room?.name} required />
      </Field>
      <Field label="Tipo" name={`${prefix}-room_type_id`} error={errors.room_type_id}>
        <Select id={`${prefix}-room_type_id`} name="room_type_id" defaultValue={room?.room_type_id ?? roomTypes[0]?.id}>
          {roomTypes.map((type) => (
            <option key={type.id} value={type.id}>
              {type.name}
            </option>
          ))}
        </Select>
      </Field>
      <Field label="Andar / ala" name={`${prefix}-floor`}>
        <Input id={`${prefix}-floor`} name="floor" defaultValue={room?.floor} />
      </Field>
      <div className="flex h-10 items-center gap-2">
        <input id={`${prefix}-active`} name="active" type="checkbox" defaultChecked={room?.active ?? true} className="size-4 rounded border-ink-300" />
        <label htmlFor={`${prefix}-active`} className="text-sm text-ink-700">
          À venda
        </label>
      </div>
      <SubmitButton variant="secondary">{room ? "Salvar" : "Adicionar quarto"}</SubmitButton>
      {state.message ? (
        <div className="sm:col-span-5">
          <FormMessage state={state} />
        </div>
      ) : null}
    </form>
  );
}
