import type { Metadata } from "next";
import Link from "next/link";

import { buttonClass } from "@/components/ui/button";
import { Card, EmptyState, PageHeader } from "@/components/ui/layout";
import { NewReservationForm } from "@/features/reservations/new-reservation-form";
import { listRoomTypes, listRooms } from "@/features/reservations/queries";
import { addDays, today } from "@/lib/format";
import { currentProperty } from "@/lib/session";

export const metadata: Metadata = { title: "Nova reserva" };

type Props = { searchParams: Promise<{ room_type_id?: string; room_id?: string; check_in?: string; check_out?: string }> };

export default async function NewReservationPage({ searchParams }: Props) {
  const property = await currentProperty();
  const params = await searchParams;
  const [roomTypes, rooms] = await Promise.all([listRoomTypes(property.id), listRooms(property.id)]);
  const activeTypes = roomTypes.filter((type) => type.active);
  const checkIn = params.check_in ?? today();

  return (
    <>
      <PageHeader title="Nova reserva" description={property.name} />
      <Card className="p-5 sm:p-6">
        {activeTypes.length === 0 ? (
          <EmptyState
            title="Cadastre os quartos primeiro"
            description="Não há tipos de acomodação ativos nesta propriedade."
            action={
              <Link href="/admin/property" className={buttonClass("primary")}>
                Ir para o cadastro
              </Link>
            }
          />
        ) : (
          <NewReservationForm
            propertyId={property.id}
            roomTypes={activeTypes}
            rooms={rooms}
            defaults={{
              roomTypeId: params.room_type_id ?? "",
              roomId: params.room_id ?? "",
              checkIn,
              checkOut: params.check_out ?? addDays(checkIn, 1),
            }}
            // Gerada no servidor a cada abertura do formulário.
            idempotencyKey={crypto.randomUUID()}
          />
        )}
      </Card>
    </>
  );
}
