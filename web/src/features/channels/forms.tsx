"use client";

import { useActionState, useState } from "react";

import { Field, FormMessage, Input, Select, SubmitButton } from "@/components/ui/form";
import { initialState } from "@/lib/action-state";
import type { ChannelConnection, RoomType } from "@/lib/types";

import { createConnection, createMapping, syncConnection, updateConnection } from "./actions";

export function NewConnectionForm({ propertyId }: { propertyId: string }) {
  const [state, action] = useActionState(createConnection, initialState);
  const [channel, setChannel] = useState("BOOKING_COM");
  const [mode, setMode] = useState("ICAL");
  const errors = state.errors ?? {};
  // A API do Airbnb é fechada a parceiros homologados: só iCal.
  const effectiveMode = channel === "AIRBNB" ? "ICAL" : mode;

  return (
    <form action={action} className="grid gap-4 sm:grid-cols-6 sm:items-end">
      <input type="hidden" name="property_id" value={propertyId} />
      <input type="hidden" name="mode" value={effectiveMode} />
      <Field label="Canal" name="channel" className="sm:col-span-2">
        <Select name="channel" value={channel} onChange={(event) => setChannel(event.target.value)}>
          <option value="BOOKING_COM">Booking.com</option>
          <option value="AIRBNB">Airbnb</option>
        </Select>
      </Field>
      <Field
        label="Integração"
        name="mode_select"
        className="sm:col-span-2"
        hint={effectiveMode === "API" ? "Exige ser parceiro certificado da Booking.com." : "Funciona com qualquer conta. Atualiza em minutos/horas."}
      >
        <Select id="mode_select" value={effectiveMode} onChange={(event) => setMode(event.target.value)} disabled={channel === "AIRBNB"}>
          <option value="ICAL">Calendário iCal</option>
          {channel === "BOOKING_COM" ? <option value="API">API de conectividade (tempo real)</option> : null}
        </Select>
      </Field>
      <Field label="Nome da conexão" name="name" className="sm:col-span-2">
        <Input name="name" placeholder="Opcional" />
      </Field>
      {effectiveMode === "API" ? (
        <>
          <Field label="ID da propriedade no canal (Hotel ID)" name="external_property_id" className="sm:col-span-2">
            <Input name="external_property_id" required />
          </Field>
          <Field label="Client ID (machine account)" name="client_id" error={errors.client_id} className="sm:col-span-2">
            <Input name="client_id" autoComplete="off" required />
          </Field>
          <Field label="Client secret" name="client_secret" className="sm:col-span-2" hint="Guardado criptografado; nunca é exibido de volta.">
            <Input name="client_secret" type="password" autoComplete="new-password" required />
          </Field>
          <Field label="Endereço da API (opcional)" name="base_url" error={errors.base_url} className="sm:col-span-4" hint="Em branco = produção da Booking.com. Em desenvolvimento: http://otasim:8090">
            <Input name="base_url" placeholder="https://..." />
          </Field>
        </>
      ) : null}
      <div className="space-y-3 sm:col-span-6">
        <FormMessage state={state} />
        <SubmitButton>Conectar canal</SubmitButton>
      </div>
    </form>
  );
}

export function MappingForm({ connection, roomTypes }: { connection: ChannelConnection; roomTypes: RoomType[] }) {
  const [state, action] = useActionState(createMapping, initialState);
  const errors = state.errors ?? {};
  const ical = connection.mode === "ICAL";
  return (
    <form action={action} className="grid gap-3 sm:grid-cols-6 sm:items-end">
      <input type="hidden" name="connection_id" value={connection.id} />
      <Field label="Tipo de acomodação" name={`room_type_id-${connection.id}`} className="sm:col-span-2">
        <Select id={`room_type_id-${connection.id}`} name="room_type_id">
          {roomTypes.map((type) => (
            <option key={type.id} value={type.id}>
              {type.name}
            </option>
          ))}
        </Select>
      </Field>
      {ical ? (
        <Field
          label="URL do calendário exportado pelo canal"
          name={`ical_import_url-${connection.id}`}
          error={errors.ical_import_url}
          hint="No Airbnb: Anúncio › Disponibilidade › Conectar calendários › Exportar."
          className="sm:col-span-3"
        >
          <Input id={`ical_import_url-${connection.id}`} name="ical_import_url" placeholder="https://..." />
        </Field>
      ) : (
        <>
          <Field label="ID do quarto no canal" name={`external_room_id-${connection.id}`} error={errors.external_room_id} className="sm:col-span-2">
            <Input id={`external_room_id-${connection.id}`} name="external_room_id" required />
          </Field>
          <Field label="ID do plano tarifário" name={`external_rate_id-${connection.id}`} className="sm:col-span-1" hint="Para enviar preços.">
            <Input id={`external_rate_id-${connection.id}`} name="external_rate_id" />
          </Field>
        </>
      )}
      <SubmitButton variant="secondary" className="sm:col-span-1">
        Mapear
      </SubmitButton>
      {state.message ? (
        <div className="sm:col-span-6">
          <FormMessage state={state} />
        </div>
      ) : null}
    </form>
  );
}

export function SyncButtons({ connection }: { connection: ChannelConnection }) {
  const [state, action] = useActionState(syncConnection, initialState);
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-2">
        <form action={action}>
          <input type="hidden" name="id" value={connection.id} />
          <input type="hidden" name="kind" value="push" />
          <SubmitButton size="sm" variant="secondary" pendingText="Enviando…">
            Reenviar disponibilidade
          </SubmitButton>
        </form>
        <form action={action}>
          <input type="hidden" name="id" value={connection.id} />
          <input type="hidden" name="kind" value="pull" />
          <SubmitButton size="sm" variant="secondary" pendingText="Consultando…">
            Buscar reservas agora
          </SubmitButton>
        </form>
      </div>
      <FormMessage state={state} />
    </div>
  );
}

export function EditConnectionForm({ connection }: { connection: ChannelConnection }) {
  const [state, action] = useActionState(updateConnection, initialState);
  const id = connection.id;
  return (
    <form action={action} className="grid gap-3 sm:grid-cols-6 sm:items-end">
      <input type="hidden" name="id" value={id} />
      <Field label="Nome" name={`name-${id}`} className="sm:col-span-2">
        <Input id={`name-${id}`} name="name" defaultValue={connection.name} />
      </Field>
      {connection.mode === "API" ? (
        <>
          <Field label="Hotel ID" name={`external_property_id-${id}`} className="sm:col-span-2">
            <Input id={`external_property_id-${id}`} name="external_property_id" defaultValue={connection.external_property_id} />
          </Field>
          <Field label="Endereço da API" name={`base_url-${id}`} className="sm:col-span-2">
            <Input id={`base_url-${id}`} name="base_url" defaultValue={connection.base_url} />
          </Field>
          <Field label="Novo client ID" name={`client_id-${id}`} className="sm:col-span-3" hint="Em branco = manter as credenciais atuais.">
            <Input id={`client_id-${id}`} name="client_id" autoComplete="off" />
          </Field>
          <Field label="Novo client secret" name={`client_secret-${id}`} className="sm:col-span-3">
            <Input id={`client_secret-${id}`} name="client_secret" type="password" autoComplete="new-password" />
          </Field>
        </>
      ) : null}
      <div className="flex items-center gap-2 sm:col-span-6">
        <input id={`active-${id}`} name="active" type="checkbox" defaultChecked={connection.active} className="size-4 rounded border-ink-300" />
        <label htmlFor={`active-${id}`} className="text-sm text-ink-700">
          Conexão ativa
        </label>
      </div>
      <div className="space-y-3 sm:col-span-6">
        <FormMessage state={state} />
        <SubmitButton size="sm" variant="secondary">
          Salvar conexão
        </SubmitButton>
      </div>
    </form>
  );
}
