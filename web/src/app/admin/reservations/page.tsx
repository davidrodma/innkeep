import type { Metadata } from "next";
import Link from "next/link";

import { buttonClass } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/form";
import { Card, EmptyState, PageHeader, StatusBadge, Table, Td, Th } from "@/components/ui/layout";
import { listReservations } from "@/features/reservations/queries";
import { formatDate, formatMoney } from "@/lib/format";
import { SOURCE_LABELS, STATUS_LABELS, label } from "@/lib/labels";
import { currentProperty } from "@/lib/session";

export const metadata: Metadata = { title: "Reservas" };

type Props = { searchParams: Promise<{ status?: string; q?: string }> };

export default async function ReservationsPage({ searchParams }: Props) {
  const property = await currentProperty();
  const { status = "", q = "" } = await searchParams;
  const reservations = await listReservations({ propertyId: property.id, status, q, limit: 200 });

  return (
    <>
      <PageHeader
        title="Reservas"
        description={`${reservations.total} reserva(s)`}
        action={
          <Link href="/admin/reservations/new" className={buttonClass("primary")}>
            Nova reserva
          </Link>
        }
      />

      {/* Formulário GET: os filtros vão para a URL, então a busca pode ser
          compartilhada, favoritada e funciona com o botão "voltar". */}
      <form method="get" className="mb-4 flex flex-wrap items-end gap-3">
        <div className="w-full sm:w-72">
          <label htmlFor="q" className="mb-1 block text-sm font-medium text-ink-700">
            Buscar
          </label>
          <Input name="q" defaultValue={q} placeholder="Nome, localizador ou nº do canal" />
        </div>
        <div className="w-full sm:w-56">
          <label htmlFor="status" className="mb-1 block text-sm font-medium text-ink-700">
            Situação
          </label>
          <Select name="status" defaultValue={status}>
            <option value="">Todas</option>
            {Object.entries(STATUS_LABELS).map(([code, text]) => (
              <option key={code} value={code}>
                {text}
              </option>
            ))}
          </Select>
        </div>
        <button type="submit" className={buttonClass("secondary")}>
          Filtrar
        </button>
      </form>

      <Card>
        {reservations.items.length === 0 ? (
          <EmptyState title="Nenhuma reserva encontrada" description="Ajuste os filtros ou crie uma nova reserva." />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Localizador</Th>
                <Th>Hóspede</Th>
                <Th>Quarto</Th>
                <Th>Entrada</Th>
                <Th>Saída</Th>
                <Th>Origem</Th>
                <Th align="right">Total</Th>
                <Th>Situação</Th>
              </tr>
            </thead>
            <tbody>
              {reservations.items.map((reservation) => (
                <tr key={reservation.id} className="hover:bg-ink-50">
                  <Td>
                    <Link href={`/admin/reservations/${reservation.id}`} className="font-mono text-xs font-medium text-brand-700 hover:underline">
                      {reservation.code}
                    </Link>
                  </Td>
                  <Td className="font-medium text-ink-900">{reservation.holder.name}</Td>
                  <Td>{reservation.room_name || "—"}</Td>
                  <Td>{formatDate(reservation.check_in)}</Td>
                  <Td>{formatDate(reservation.check_out)}</Td>
                  <Td>{label(SOURCE_LABELS, reservation.source)}</Td>
                  <Td align="right">{formatMoney(reservation.total_cents)}</Td>
                  <Td>
                    <StatusBadge status={reservation.status} />
                  </Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>
    </>
  );
}
