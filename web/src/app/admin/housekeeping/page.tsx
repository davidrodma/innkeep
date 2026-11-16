import type { Metadata } from "next";

import { Badge, Card, EmptyState, PageHeader } from "@/components/ui/layout";
import { setHousekeeping } from "@/features/property/actions";
import { listReservations, listRooms } from "@/features/reservations/queries";
import { addDays, today } from "@/lib/format";
import { currentProperty } from "@/lib/session";

export const metadata: Metadata = { title: "Governança" };

export default async function HousekeepingPage() {
  const property = await currentProperty();
  const date = today();
  // A governança não tem acesso às reservas (dados de hóspedes): nesse caso
  // a tela mostra só a limpeza, sem a ocupação.
  const empty = { items: [], total: 0 };
  const [rooms, inHouse, arrivals] = await Promise.all([
    listRooms(property.id),
    listReservations({ propertyId: property.id, status: "CHECKED_IN", limit: 200 }).catch(() => empty),
    listReservations({ propertyId: property.id, from: date, to: addDays(date, 1), status: "CONFIRMED,PENDING", limit: 200 }).catch(() => empty),
  ]);
  const occupied = new Set(inHouse.items.map((reservation) => reservation.room_id));
  const arrivingToday = new Set(arrivals.items.filter((reservation) => reservation.check_in === date).map((reservation) => reservation.room_id));

  return (
    <>
      <PageHeader
        title="Governança"
        description="Situação de limpeza dos quartos. Todo check-out marca o quarto como sujo automaticamente."
      />
      {rooms.length === 0 ? (
        <Card>
          <EmptyState title="Nenhum quarto cadastrado" />
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {rooms.map((room) => {
            const dirty = room.housekeeping === "DIRTY";
            return (
              <Card key={room.id} className={`p-4 ${dirty ? "border-amber-300" : ""}`}>
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <p className="text-lg font-semibold text-ink-900">{room.name}</p>
                    <div className="mt-1 flex flex-wrap gap-1">
                      {occupied.has(room.id) ? <Badge tone="blue">Ocupado</Badge> : <Badge>Vago</Badge>}
                      {arrivingToday.has(room.id) ? <Badge tone="violet">Chegada hoje</Badge> : null}
                    </div>
                  </div>
                  <Badge tone={dirty ? "amber" : "brand"}>{dirty ? "Sujo" : "Limpo"}</Badge>
                </div>
                <form action={setHousekeeping} className="mt-4">
                  <input type="hidden" name="room_id" value={room.id} />
                  <input type="hidden" name="status" value={dirty ? "CLEAN" : "DIRTY"} />
                  <button
                    type="submit"
                    className={`h-9 w-full rounded-lg text-sm font-medium ${
                      dirty ? "bg-brand-600 text-white hover:bg-brand-700" : "border border-ink-300 text-ink-700 hover:bg-ink-100"
                    }`}
                  >
                    {dirty ? "Marcar como limpo" : "Marcar como sujo"}
                  </button>
                </form>
              </Card>
            );
          })}
        </div>
      )}
    </>
  );
}
