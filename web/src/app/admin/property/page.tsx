import type { Metadata } from "next";
import Link from "next/link";

import { buttonClass } from "@/components/ui/button";
import { Badge, Card, CardHeader, EmptyState, PageHeader } from "@/components/ui/layout";
import { PropertyForm } from "@/features/property/property-form";
import { RoomForm, RoomTypeForm } from "@/features/property/room-forms";
import { listRoomTypes, listRooms } from "@/features/reservations/queries";
import { formatMoney } from "@/lib/format";
import { currentProperty } from "@/lib/session";

export const metadata: Metadata = { title: "Propriedade" };

export default async function PropertyPage() {
  const property = await currentProperty();
  const [roomTypes, rooms] = await Promise.all([listRoomTypes(property.id), listRooms(property.id)]);

  return (
    <>
      <PageHeader
        title={property.name}
        description="Dados, políticas, tipos de acomodação e quartos."
        action={
          <>
            {property.published ? (
              <Link href={`/p/${property.slug}`} className={buttonClass("secondary", "sm")} target="_blank">
                Ver no site
              </Link>
            ) : null}
            <Link href="/admin/property/new" className={buttonClass("ghost", "sm")}>
              Cadastrar outra propriedade
            </Link>
          </>
        }
      />

      <div className="space-y-6">
        <Card>
          <CardHeader
            title="Tipos de acomodação"
            description="É o tipo — não o quarto físico — que se vende no site e nos canais, e que tem preço."
          />
          <div className="divide-y divide-ink-100">
            {roomTypes.map((roomType) => (
              <details key={roomType.id} className="group">
                <summary className="flex cursor-pointer flex-wrap items-center justify-between gap-2 px-5 py-3 hover:bg-ink-50">
                  <span className="text-sm font-medium text-ink-900">
                    {roomType.name}{" "}
                    <span className="font-normal text-ink-500">
                      · até {roomType.max_occupancy} pessoa(s) · {formatMoney(roomType.base_price_cents)}/noite ·{" "}
                      {rooms.filter((room) => room.room_type_id === roomType.id).length} quarto(s)
                    </span>
                  </span>
                  {roomType.active ? <Badge tone="brand">Ativo</Badge> : <Badge>Inativo</Badge>}
                </summary>
                <div className="bg-ink-50/50 px-5 py-4">
                  <RoomTypeForm propertyId={property.id} roomType={roomType} />
                </div>
              </details>
            ))}
            <details open={roomTypes.length === 0}>
              <summary className="cursor-pointer px-5 py-3 text-sm font-medium text-brand-700 hover:bg-ink-50">
                + Novo tipo de acomodação
              </summary>
              <div className="px-5 py-4">
                <RoomTypeForm propertyId={property.id} />
              </div>
            </details>
          </div>
        </Card>

        <Card>
          <CardHeader title="Quartos" description="As unidades físicas. Em hostels, cadastre cada cama como uma unidade." />
          {roomTypes.length === 0 ? (
            <EmptyState title="Cadastre um tipo de acomodação primeiro" />
          ) : (
            <div className="space-y-4 px-5 py-4">
              {rooms.map((room) => (
                <RoomForm key={room.id} propertyId={property.id} roomTypes={roomTypes} room={room} />
              ))}
              <div className="border-t border-ink-200 pt-4">
                <RoomForm propertyId={property.id} roomTypes={roomTypes} />
              </div>
            </div>
          )}
        </Card>

        <Card>
          <CardHeader title="Dados e políticas" />
          <div className="px-5 py-5">
            <PropertyForm property={property} />
          </div>
        </Card>
      </div>
    </>
  );
}
