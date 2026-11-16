import type { Metadata } from "next";

import { MonthNav } from "@/components/admin/month-nav";
import { Badge, Card, CardHeader, EmptyState, Stat, Table, Td, Th } from "@/components/ui/layout";
import { CashForm } from "@/features/finance/forms";
import { monthPeriod } from "@/features/finance/period";
import { api, query } from "@/lib/api";
import { formatDate, formatMoney, today } from "@/lib/format";
import { currentProperty } from "@/lib/session";
import type { CashMovement, FinancialReport, List } from "@/lib/types";

export const metadata: Metadata = { title: "Caixa" };

type Props = { searchParams: Promise<{ month?: string }> };

const ORIGIN: Record<string, string> = {
  PAYMENT: "Recebimento",
  REFUND: "Devolução",
  EXPENSE: "Despesa",
  WITHDRAWAL: "Sangria",
  DEPOSIT: "Suprimento",
  ADJUSTMENT: "Ajuste",
};

export default async function CashPage({ searchParams }: Props) {
  const property = await currentProperty();
  const period = monthPeriod((await searchParams).month);
  const range = query({ property_id: property.id, from: period.from, to: period.to });
  const [movements, report] = await Promise.all([
    api<List<CashMovement>>(`/v1/cash/movements${range}`),
    api<FinancialReport>(`/v1/reports/financial${range}`),
  ]);

  return (
    <>
      <MonthNav path="/admin/finance/cash" label={period.label} previousMonth={period.previousMonth} nextMonth={period.nextMonth} />
      <div className="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Saldo inicial" value={formatMoney(report.cash.opening_cents)} />
        <Stat label="Entradas" value={formatMoney(report.cash.in_cents)} />
        <Stat label="Saídas" value={formatMoney(report.cash.out_cents)} />
        <Stat label="Saldo em caixa" value={formatMoney(report.cash.closing_cents)} tone={report.cash.closing_cents >= 0 ? "neutral" : "bad"} />
      </div>

      <Card className="mb-6">
        <CardHeader
          title="Sangria ou suprimento"
          description="Recebimentos e despesas em dinheiro entram no caixa automaticamente; aqui ficam as retiradas e os reforços."
        />
        <div className="px-5 py-4">
          <CashForm propertyId={property.id} today={today()} />
        </div>
      </Card>

      <Card>
        <CardHeader title="Movimentos do mês" />
        {movements.items.length === 0 ? (
          <EmptyState title="Nenhum movimento de caixa neste mês" />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Data</Th>
                <Th>Origem</Th>
                <Th>Descrição</Th>
                <Th align="right">Valor</Th>
              </tr>
            </thead>
            <tbody>
              {movements.items.map((movement) => (
                <tr key={movement.id}>
                  <Td>{formatDate(movement.occurred_on)}</Td>
                  <Td>
                    <Badge tone={movement.kind === "IN" ? "brand" : "red"}>{ORIGIN[movement.ref_type] ?? movement.ref_type}</Badge>
                  </Td>
                  <Td>{movement.reason}</Td>
                  <Td align="right" className={movement.kind === "IN" ? "text-brand-700" : "text-red-700"}>
                    {movement.kind === "IN" ? "+ " : "− "}
                    {formatMoney(movement.amount_cents)}
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
