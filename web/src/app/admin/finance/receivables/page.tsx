import type { Metadata } from "next";
import Link from "next/link";

import { Card, CardHeader, EmptyState, Table, Td, Th } from "@/components/ui/layout";
import { api, query } from "@/lib/api";
import { formatDate, formatMoney } from "@/lib/format";
import { SOURCE_LABELS, label } from "@/lib/labels";
import { currentProperty } from "@/lib/session";
import type { FolioSummary, List } from "@/lib/types";

export const metadata: Metadata = { title: "Contas a receber" };

export default async function ReceivablesPage() {
  const property = await currentProperty();
  const folios = await api<List<FolioSummary>>(`/v1/folios${query({ property_id: property.id, outstanding: true, limit: 200 })}`);
  const total = folios.items.reduce((sum, folio) => sum + folio.balance_cents, 0);

  return (
    <Card>
      <CardHeader title="Contas com saldo em aberto" description={`${folios.total} conta(s) · ${formatMoney(total)} a receber`} />
      {folios.items.length === 0 ? (
        <EmptyState title="Nenhum saldo a receber" description="Todas as contas estão quitadas." />
      ) : (
        <Table>
          <thead>
            <tr>
              <Th>Reserva</Th>
              <Th>Hóspede</Th>
              <Th>Estadia</Th>
              <Th>Canal</Th>
              <Th align="right">Total</Th>
              <Th align="right">Pago</Th>
              <Th align="right">Saldo</Th>
            </tr>
          </thead>
          <tbody>
            {folios.items.map((folio) => (
              <tr key={folio.id} className="hover:bg-ink-50">
                <Td>
                  <Link href={`/admin/reservations/${folio.reservation_id}`} className="font-mono text-xs font-medium text-brand-700 hover:underline">
                    {folio.reservation_code}
                  </Link>
                </Td>
                <Td>{folio.guest_name}</Td>
                <Td>
                  {formatDate(folio.check_in)} a {formatDate(folio.check_out)}
                </Td>
                <Td>{label(SOURCE_LABELS, folio.source)}</Td>
                <Td align="right">{formatMoney(folio.total_due_cents)}</Td>
                <Td align="right">{formatMoney(folio.paid_cents)}</Td>
                <Td align="right" className="font-semibold text-amber-700">
                  {formatMoney(folio.balance_cents)}
                </Td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </Card>
  );
}
