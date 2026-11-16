import type { Metadata } from "next";

import { Badge, Card, CardHeader, EmptyState, PageHeader } from "@/components/ui/layout";
import { deleteConnection, deleteMapping } from "@/features/channels/actions";
import { EditConnectionForm, MappingForm, NewConnectionForm, SyncButtons } from "@/features/channels/forms";
import { listRoomTypes } from "@/features/reservations/queries";
import { api, query } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { SOURCE_LABELS, label } from "@/lib/labels";
import { currentProperty } from "@/lib/session";
import type { ChannelConnection, ChannelMapping, List, SyncLog } from "@/lib/types";

export const metadata: Metadata = { title: "Canais de venda" };

export default async function ChannelsPage() {
  const property = await currentProperty();
  const [connections, roomTypes] = await Promise.all([
    api<List<ChannelConnection>>(`/v1/channels/connections${query({ property_id: property.id })}`),
    listRoomTypes(property.id),
  ]);
  const details = await Promise.all(
    connections.items.map(async (connection) => ({
      connection,
      mappings: (await api<List<ChannelMapping>>(`/v1/channels/connections/${connection.id}/mappings`)).items,
      logs: (await api<List<SyncLog>>(`/v1/channels/connections/${connection.id}/sync-log?limit=8`)).items,
    })),
  );
  const typeName = (id: string) => roomTypes.find((type) => type.id === id)?.name ?? "Tipo removido";

  return (
    <>
      <PageHeader
        title="Canais de venda"
        description="Booking.com e Airbnb. Toda mudança de disponibilidade no InnKeep é replicada para os canais conectados."
      />

      <div className="space-y-6">
        {details.map(({ connection, mappings, logs }) => (
          <Card key={connection.id}>
            <CardHeader
              title={connection.name}
              description={`${label(SOURCE_LABELS, connection.channel)} · ${connection.mode === "API" ? "API de conectividade" : "Calendário iCal"}`}
              action={
                <div className="flex items-center gap-2">
                  {connection.last_error ? <Badge tone="red">Com erro</Badge> : connection.active ? <Badge tone="brand">Ativa</Badge> : <Badge>Inativa</Badge>}
                </div>
              }
            />
            <div className="space-y-5 px-5 py-4">
              <p className="text-xs text-ink-500">
                Último envio: {connection.last_push_at ? formatDateTime(connection.last_push_at) : "nunca"} · Última consulta de reservas:{" "}
                {connection.last_pull_at ? formatDateTime(connection.last_pull_at) : "nunca"}
              </p>
              {connection.last_error ? <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{connection.last_error}</p> : null}

              <div>
                <h3 className="mb-2 text-sm font-semibold text-ink-900">Tipos de acomodação mapeados</h3>
                {mappings.length === 0 ? (
                  <p className="mb-3 text-sm text-ink-500">Nenhum. Sem mapeamento, nada é sincronizado.</p>
                ) : (
                  <ul className="mb-3 divide-y divide-ink-100 rounded-lg border border-ink-200">
                    {mappings.map((mapping) => (
                      <li key={mapping.id} className="space-y-1 px-4 py-2.5 text-sm">
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <span className="font-medium text-ink-900">
                            {typeName(mapping.room_type_id)}
                            {mapping.external_room_id ? <span className="font-normal text-ink-500"> → quarto {mapping.external_room_id}</span> : null}
                          </span>
                          <form action={deleteMapping}>
                            <input type="hidden" name="id" value={mapping.id} />
                            <button type="submit" className="text-xs text-ink-500 hover:text-red-700">
                              Remover
                            </button>
                          </form>
                        </div>
                        {connection.mode === "ICAL" ? (
                          <div className="text-xs text-ink-600">
                            <p>Cole este endereço no canal, em “importar calendário”:</p>
                            <code className="mt-1 block overflow-x-auto rounded bg-ink-100 px-2 py-1 font-mono text-[11px] text-ink-800">
                              {mapping.ical_export_url}
                            </code>
                          </div>
                        ) : null}
                      </li>
                    ))}
                  </ul>
                )}
                {roomTypes.length > 0 ? <MappingForm connection={connection} roomTypes={roomTypes} /> : null}
              </div>

              <SyncButtons connection={connection} />

              {logs.length > 0 ? (
                <details>
                  <summary className="cursor-pointer text-sm font-medium text-ink-700">Histórico de sincronização</summary>
                  <ul className="mt-2 space-y-1 text-xs">
                    {logs.map((log) => (
                      <li key={log.id} className={log.success ? "text-ink-600" : "text-red-700"}>
                        {formatDateTime(log.created_at)} · {log.direction === "OUTBOUND" ? "enviado" : "recebido"} · {log.operation} · {log.detail}
                      </li>
                    ))}
                  </ul>
                </details>
              ) : null}

              <details>
                <summary className="cursor-pointer text-sm font-medium text-ink-700">Configurações da conexão</summary>
                <div className="mt-3 space-y-4">
                  <EditConnectionForm connection={connection} />
                  <form action={deleteConnection}>
                    <input type="hidden" name="id" value={connection.id} />
                    <button type="submit" className="text-sm text-red-700 hover:underline">
                      Excluir conexão
                    </button>
                  </form>
                </div>
              </details>
            </div>
          </Card>
        ))}

        <Card>
          <CardHeader title="Conectar novo canal" />
          <div className="px-5 py-4">
            {connections.items.length === 0 && roomTypes.length === 0 ? (
              <EmptyState title="Cadastre os tipos de acomodação antes de conectar um canal" />
            ) : (
              <NewConnectionForm propertyId={property.id} />
            )}
          </div>
        </Card>
      </div>
    </>
  );
}
