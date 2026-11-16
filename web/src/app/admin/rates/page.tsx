import type { Metadata } from "next";
import Link from "next/link";

import { buttonClass } from "@/components/ui/button";
import { Card, CardHeader, EmptyState, PageHeader } from "@/components/ui/layout";
import { deleteBlock } from "@/features/rates/actions";
import { BlockForm, RatesForm } from "@/features/rates/forms";
import { getAvailability, listBlocks, listRoomTypes, listRooms } from "@/features/reservations/queries";
import { addDays, eachDay, formatDate, formatDayMonth, formatMoney, isWeekend, today, weekday } from "@/lib/format";
import { currentProperty } from "@/lib/session";

export const metadata: Metadata = { title: "Tarifas e disponibilidade" };

const DAYS = 14;

type Props = { searchParams: Promise<{ from?: string }> };

export default async function RatesPage({ searchParams }: Props) {
  const property = await currentProperty();
  const params = await searchParams;
  const from = /^\d{4}-\d{2}-\d{2}$/.test(params.from ?? "") ? params.from! : today();
  const to = addDays(from, DAYS);

  const [grid, roomTypes, rooms, blocks] = await Promise.all([
    getAvailability(property.id, from, to),
    listRoomTypes(property.id),
    listRooms(property.id),
    listBlocks(property.id, today(), addDays(today(), 365)),
  ]);
  const days = eachDay(from, to);

  return (
    <>
      <PageHeader
        title="Tarifas e disponibilidade"
        description="Preço, estadia mínima e vendas por data. As mudanças vão automaticamente para os canais conectados."
        action={
          <>
            <Link href={`/admin/rates?from=${addDays(from, -DAYS)}`} className={buttonClass("secondary", "sm")}>
              ←
            </Link>
            <Link href="/admin/rates" className={buttonClass("secondary", "sm")}>
              Hoje
            </Link>
            <Link href={`/admin/rates?from=${addDays(from, DAYS)}`} className={buttonClass("secondary", "sm")}>
              →
            </Link>
          </>
        }
      />

      <Card className="mb-6 overflow-x-auto">
        {grid.room_types.length === 0 ? (
          <EmptyState title="Nenhum tipo de acomodação ativo" />
        ) : (
          <table className="w-full min-w-[56rem] text-center text-xs">
            <thead>
              <tr className="border-b border-ink-200">
                <th className="w-44 px-3 py-2 text-left font-medium text-ink-500">Tipo</th>
                {days.map((day) => (
                  <th key={day} className={`px-1 py-2 font-medium ${isWeekend(day) ? "bg-ink-50 text-ink-700" : "text-ink-500"}`}>
                    <div className="uppercase">{weekday(day)}</div>
                    <div className="tabular">{formatDayMonth(day)}</div>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {grid.room_types.map((roomType) => (
                <tr key={roomType.room_type_id} className="border-b border-ink-100">
                  <th scope="row" className="px-3 py-2 text-left text-sm font-medium text-ink-800">
                    {roomType.name}
                  </th>
                  {roomType.days.map((day) => (
                    <td
                      key={day.date}
                      title={`${formatDate(day.date)}: ${day.available} de ${day.total} livre(s)${day.closed ? " · vendas fechadas" : ""}${day.min_stay > 1 ? ` · mínimo ${day.min_stay} noites` : ""}`}
                      className={`px-1 py-2 ${day.closed ? "bg-danger-soft" : isWeekend(day.date) ? "bg-ink-50" : ""}`}
                    >
                      <div className={`tabular font-semibold ${day.available === 0 ? "text-danger-ink" : "text-ink-900"}`}>
                        {day.available}/{day.total}
                      </div>
                      <div className="tabular text-ink-600">{formatMoney(day.price_cents).replace("R$", "").trim()}</div>
                      {day.min_stay > 1 ? <div className="text-[10px] text-warning-ink">mín {day.min_stay}</div> : null}
                      {day.closed ? <div className="text-[10px] font-semibold text-danger-ink">fechado</div> : null}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>

      {roomTypes.length > 0 ? (
        <div className="grid gap-6">
          <Card>
            <CardHeader title="Alterar tarifas e restrições" />
            <div className="px-5 py-4">
              <RatesForm roomTypes={roomTypes.filter((type) => type.active)} from={from} to={addDays(to, -1)} />
            </div>
          </Card>

          <Card>
            <CardHeader title="Bloqueios de quarto" description="Tira um quarto de venda: manutenção, uso do proprietário etc." />
            <div className="space-y-4 px-5 py-4">
              {blocks.length > 0 ? (
                <ul className="divide-y divide-ink-100 rounded-lg border border-ink-200">
                  {blocks.map((block) => (
                    <li key={block.id} className="flex flex-wrap items-center justify-between gap-2 px-4 py-2.5 text-sm">
                      <span>
                        Quarto <strong>{block.room_name}</strong> · {formatDate(block.from)} a {formatDate(addDays(block.to, -1))}
                        {block.reason ? <span className="text-ink-500"> · {block.reason}</span> : null}
                      </span>
                      <form action={deleteBlock}>
                        <input type="hidden" name="id" value={block.id} />
                        <button type="submit" className="text-xs text-ink-500 hover:text-danger-ink">
                          Remover bloqueio
                        </button>
                      </form>
                    </li>
                  ))}
                </ul>
              ) : null}
              <BlockForm rooms={rooms.filter((room) => room.active)} from={today()} to={today()} />
            </div>
          </Card>
        </div>
      ) : null}
    </>
  );
}
