import type { Metadata } from "next";
import Link from "next/link";

import { buttonClass } from "@/components/ui/button";
import { Card, EmptyState, PageHeader, STATUS_TONES } from "@/components/ui/layout";
import { placeStay } from "@/features/calendar/layout";
import { listBlocks, listReservations, listRoomTypes, listRooms } from "@/features/reservations/queries";
import { addDays, eachDay, formatDate, formatDayMonth, isWeekend, today, weekday } from "@/lib/format";
import { STATUS_LABELS } from "@/lib/labels";
import { currentProperty } from "@/lib/session";
import type { Block, Reservation, Room } from "@/lib/types";

export const metadata: Metadata = { title: "Calendário" };

const DAYS = 14;
// Situações que ocupam (ou podem ocupar) um quarto.
const VISIBLE_STATUSES = "REQUESTED,PENDING,CONFIRMED,CHECKED_IN,CHECKED_OUT";

const BAR_COLORS: Record<string, string> = {
  neutral: "bg-ink-200 text-ink-800 hover:bg-ink-300",
  amber: "bg-amber-200 text-amber-900 hover:bg-amber-300",
  brand: "bg-brand-500 text-white hover:bg-brand-600",
  blue: "bg-sky-500 text-white hover:bg-sky-600",
  red: "bg-red-200 text-red-900",
  violet: "bg-violet-200 text-violet-900",
};

type Props = { searchParams: Promise<{ from?: string }> };

export default async function CalendarPage({ searchParams }: Props) {
  const property = await currentProperty();
  const params = await searchParams;
  const from = /^\d{4}-\d{2}-\d{2}$/.test(params.from ?? "") ? params.from! : addDays(today(), -1);
  const to = addDays(from, DAYS);
  const days = eachDay(from, to);
  const currentDay = today();

  // Três serviços diferentes, consultados em paralelo: catálogo (property),
  // reservas (reservation) e bloqueios (inventory).
  const [roomTypes, rooms, reservationList, blocks] = await Promise.all([
    listRoomTypes(property.id),
    listRooms(property.id),
    listReservations({ propertyId: property.id, from, to, status: VISIBLE_STATUSES, limit: 500 }),
    listBlocks(property.id, from, to),
  ]);
  const reservations = reservationList.items;

  const gridTemplate = { gridTemplateColumns: `9rem repeat(${DAYS}, minmax(3.25rem, 1fr))` };

  return (
    <>
      <PageHeader
        title="Calendário"
        description={`${formatDate(from)} a ${formatDate(addDays(to, -1))}`}
        action={
          <>
            <Link href={`/admin/calendar?from=${addDays(from, -7)}`} className={buttonClass("secondary", "sm")}>
              ← Semana anterior
            </Link>
            <Link href="/admin/calendar" className={buttonClass("secondary", "sm")}>
              Hoje
            </Link>
            <Link href={`/admin/calendar?from=${addDays(from, 7)}`} className={buttonClass("secondary", "sm")}>
              Próxima semana →
            </Link>
          </>
        }
      />

      {rooms.length === 0 ? (
        <Card>
          <EmptyState
            title="Nenhum quarto cadastrado"
            description="Cadastre os tipos de acomodação e os quartos para começar a receber reservas."
            action={
              <Link href="/admin/property" className={buttonClass("primary")}>
                Cadastrar quartos
              </Link>
            }
          />
        </Card>
      ) : (
        <Card className="overflow-x-auto">
          <div className="min-w-[60rem]">
            {/* Cabeçalho com os dias */}
            <div className="grid border-b border-ink-200 text-xs" style={gridTemplate}>
              <div className="px-3 py-2 font-medium text-ink-500">Quarto</div>
              {days.map((day) => (
                <div
                  key={day}
                  className={`border-l border-ink-100 px-1 py-2 text-center ${
                    day === currentDay ? "bg-brand-50 font-semibold text-brand-800" : isWeekend(day) ? "bg-ink-50 text-ink-600" : "text-ink-600"
                  }`}
                >
                  <div className="uppercase">{weekday(day)}</div>
                  <div className="tabular">{formatDayMonth(day)}</div>
                </div>
              ))}
            </div>

            {roomTypes.map((roomType) => {
              const typeRooms = rooms.filter((room) => room.room_type_id === roomType.id);
              const unassigned = reservations.filter((r) => r.room_type_id === roomType.id && !r.room_id);
              if (typeRooms.length === 0) return null;
              return (
                <div key={roomType.id}>
                  <div className="border-b border-ink-200 bg-ink-50 px-3 py-1.5 text-xs font-semibold text-ink-700">
                    {roomType.name}
                  </div>
                  {typeRooms.map((room) => (
                    <RoomRow
                      key={room.id}
                      room={room}
                      roomTypeId={roomType.id}
                      days={days}
                      from={from}
                      currentDay={currentDay}
                      gridTemplate={gridTemplate}
                      reservations={reservations.filter((r) => r.room_id === room.id)}
                      blocks={blocks.filter((block) => block.room_id === room.id)}
                    />
                  ))}
                  {unassigned.length > 0 ? (
                    <RoomRow
                      room={null}
                      roomTypeId={roomType.id}
                      days={days}
                      from={from}
                      currentDay={currentDay}
                      gridTemplate={gridTemplate}
                      reservations={unassigned}
                      blocks={[]}
                    />
                  ) : null}
                </div>
              );
            })}
          </div>
        </Card>
      )}

      <ul className="mt-4 flex flex-wrap gap-x-4 gap-y-2 text-xs text-ink-600">
        {(["PENDING", "CONFIRMED", "CHECKED_IN", "CHECKED_OUT"] as const).map((status) => (
          <li key={status} className="flex items-center gap-1.5">
            <span className={`size-3 rounded-sm ${BAR_COLORS[STATUS_TONES[status]]?.split(" ")[0]}`} aria-hidden />
            {STATUS_LABELS[status]}
          </li>
        ))}
        <li className="flex items-center gap-1.5">
          <span className="size-3 rounded-sm bg-ink-400" aria-hidden />
          Bloqueado
        </li>
      </ul>
    </>
  );
}

function RoomRow({
  room,
  roomTypeId,
  days,
  from,
  currentDay,
  gridTemplate,
  reservations,
  blocks,
}: {
  room: Room | null;
  roomTypeId: string;
  days: string[];
  from: string;
  currentDay: string;
  gridTemplate: React.CSSProperties;
  reservations: Reservation[];
  blocks: Block[];
}) {
  return (
    // As células de fundo e as barras ocupam a MESMA grade: as barras são
    // posicionadas por gridColumn e ficam por cima das células.
    <div className="relative grid border-b border-ink-100" style={gridTemplate}>
      <div className={`flex items-center px-3 py-2 text-sm ${room ? "text-ink-800" : "font-medium text-red-700"}`} style={{ gridColumn: 1, gridRow: 1 }}>
        {room ? room.name : "Sem quarto"}
        {room && !room.active ? <span className="ml-2 text-xs text-ink-400">(fora de venda)</span> : null}
      </div>

      {days.map((day, index) => (
        <Link
          key={day}
          href={`/admin/reservations/new?room_type_id=${roomTypeId}${room ? `&room_id=${room.id}` : ""}&check_in=${day}&check_out=${addDays(day, 1)}`}
          aria-label={`Nova reserva em ${formatDate(day)}${room ? `, quarto ${room.name}` : ""}`}
          className={`h-11 border-l border-ink-100 hover:bg-brand-50 ${
            day === currentDay ? "bg-brand-50/60" : isWeekend(day) ? "bg-ink-50/70" : ""
          }`}
          style={{ gridColumn: index + 2, gridRow: 1 }}
        />
      ))}

      {blocks.map((block) => {
        const span = placeStay(block.from, block.to, from, days.length);
        if (!span) return null;
        return (
          <div
            key={block.id}
            title={`Bloqueado: ${block.reason || "sem motivo informado"}`}
            className="z-10 m-1 flex items-center overflow-hidden rounded-md bg-ink-400 px-2 text-xs font-medium text-white"
            style={{ gridColumn: `${span.column + 1} / span ${span.length}`, gridRow: 1 }}
          >
            <span className="truncate">{block.reason || "Bloqueado"}</span>
          </div>
        );
      })}

      {reservations.map((reservation) => {
        const span = placeStay(reservation.check_in, reservation.check_out, from, days.length);
        if (!span) return null;
        return (
          <Link
            key={reservation.id}
            href={`/admin/reservations/${reservation.id}`}
            title={`${reservation.holder.name} · ${formatDate(reservation.check_in)} a ${formatDate(reservation.check_out)} · ${STATUS_LABELS[reservation.status]}`}
            className={`z-10 m-1 flex items-center overflow-hidden px-2 text-xs font-medium ${
              BAR_COLORS[STATUS_TONES[reservation.status]]
            } ${span.startsBefore ? "rounded-l-none" : "rounded-l-md"} ${span.endsAfter ? "rounded-r-none" : "rounded-r-md"}`}
            style={{ gridColumn: `${span.column + 1} / span ${span.length}`, gridRow: 1 }}
          >
            <span className="truncate">{reservation.holder.name}</span>
          </Link>
        );
      })}
    </div>
  );
}
