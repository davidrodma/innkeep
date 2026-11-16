import Link from "next/link";
import { redirect } from "next/navigation";

import { buttonClass } from "@/components/ui/button";
import { Card, CardHeader, EmptyState, PageHeader, Stat, StatusBadge } from "@/components/ui/layout";
import { getAvailability, getDashboard } from "@/features/reservations/queries";
import { addDays, formatDate, formatMoney, today } from "@/lib/format";
import { can, currentProperty, currentUser } from "@/lib/session";
import type { Reservation } from "@/lib/types";

export default async function DashboardPage() {
  const user = await currentUser();
  // O painel mostra reservas: quem não pode vê-las vai direto à sua tela.
  if (!can(user, "reservation:read")) redirect(can(user, "housekeeping:write") ? "/admin/housekeeping" : "/admin/users");
  const property = await currentProperty();
  const date = today();
  const [dashboard, availability] = await Promise.all([
    getDashboard(property.id, date),
    getAvailability(property.id, date, addDays(date, 1)),
  ]);

  const totalRooms = availability.room_types.reduce((sum, type) => sum + (type.days[0]?.total ?? 0), 0);
  const freeRooms = availability.room_types.reduce((sum, type) => sum + (type.days[0]?.available ?? 0), 0);
  const occupancy = totalRooms > 0 ? Math.round(((totalRooms - freeRooms) / totalRooms) * 100) : 0;
  const arrivalsPending = dashboard.arrivals.filter((reservation) => reservation.status !== "CHECKED_IN");
  const departuresPending = dashboard.departures.filter((reservation) => reservation.status === "CHECKED_IN");

  return (
    <>
      <PageHeader
        title="Painel"
        description={`${property.name} · ${formatDate(date)}`}
        action={
          <Link href="/admin/reservations/new" className={buttonClass("primary")}>
            Nova reserva
          </Link>
        }
      />

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Chegadas hoje" value={String(arrivalsPending.length)} hint={`${dashboard.arrivals.length} previstas`} />
        <Stat label="Saídas hoje" value={String(departuresPending.length)} hint={`${dashboard.departures.length} previstas`} />
        <Stat label="Hospedados agora" value={String(dashboard.in_house.length)} />
        <Stat label="Ocupação hoje" value={`${occupancy}%`} hint={`${totalRooms - freeRooms} de ${totalRooms} quartos`} />
      </div>

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <ReservationList title="Chegadas de hoje" empty="Nenhuma chegada prevista para hoje." reservations={dashboard.arrivals} />
        <ReservationList title="Saídas de hoje" empty="Nenhuma saída prevista para hoje." reservations={dashboard.departures} />
        <ReservationList
          title="Aguardando sinal"
          description="Reservas que expiram se o sinal não for pago no prazo."
          empty="Nenhuma reserva aguardando pagamento."
          reservations={dashboard.pending}
        />
        <ReservationList title="Hospedados" empty="Ninguém hospedado no momento." reservations={dashboard.in_house} />
      </div>
    </>
  );
}

function ReservationList({
  title,
  description,
  empty,
  reservations,
}: {
  title: string;
  description?: string;
  empty: string;
  reservations: Reservation[];
}) {
  return (
    <Card>
      <CardHeader title={title} description={description} />
      {reservations.length === 0 ? (
        <EmptyState title={empty} />
      ) : (
        <ul className="divide-y divide-ink-100">
          {reservations.map((reservation) => (
            <li key={reservation.id}>
              <Link
                href={`/admin/reservations/${reservation.id}`}
                className="flex items-center justify-between gap-3 px-5 py-3 hover:bg-ink-50"
              >
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-ink-900">{reservation.holder.name}</p>
                  <p className="text-xs text-ink-500">
                    {reservation.room_name ? `Quarto ${reservation.room_name}` : "Sem quarto"} ·{" "}
                    {formatDate(reservation.check_in)} a {formatDate(reservation.check_out)} ·{" "}
                    {formatMoney(reservation.total_cents)}
                  </p>
                </div>
                <StatusBadge status={reservation.status} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
