import type { Metadata } from "next";

import { MonthNav } from "@/components/admin/month-nav";
import { Card, CardHeader, EmptyState, Stat, Table, Td, Th } from "@/components/ui/layout";
import { computeKpis } from "@/features/finance/kpis";
import { monthPeriod } from "@/features/finance/period";
import { getAvailability, listReservations } from "@/features/reservations/queries";
import { api, query } from "@/lib/api";
import { formatDayMonth, formatMoney } from "@/lib/format";
import { EXPENSE_CATEGORY_LABELS, METHOD_LABELS, SOURCE_LABELS, label } from "@/lib/labels";
import { currentProperty } from "@/lib/session";
import type { FinancialReport } from "@/lib/types";

export const metadata: Metadata = { title: "Financeiro" };

type Props = { searchParams: Promise<{ month?: string }> };

export default async function FinanceSummaryPage({ searchParams }: Props) {
  const property = await currentProperty();
  const period = monthPeriod((await searchParams).month);

  const [report, grid, reservations] = await Promise.all([
    api<FinancialReport>(`/v1/reports/financial${query({ property_id: property.id, from: period.from, to: period.to })}`),
    getAvailability(property.id, period.from, period.to),
    listReservations({ propertyId: property.id, from: period.from, to: period.to, status: "CONFIRMED,CHECKED_IN,CHECKED_OUT", limit: 500 }),
  ]);
  const kpis = computeKpis(grid, reservations.items, period.from, period.to);
  const maxDaily = Math.max(1, ...report.daily.map((day) => Math.max(day.gross_cents, day.expenses_cents)));

  return (
    <>
      <MonthNav path="/admin/finance" label={period.label} previousMonth={period.previousMonth} nextMonth={period.nextMonth} />

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Recebido (bruto)" value={formatMoney(report.gross_cents)} hint={`Taxas: ${formatMoney(report.fees_cents)}`} />
        <Stat label="Recebido (líquido)" value={formatMoney(report.net_cents)} hint="Bruto − taxas − devoluções" />
        <Stat label="Despesas pagas" value={formatMoney(report.expenses_paid_cents)} hint={`A pagar: ${formatMoney(report.expenses_pending_cents)}`} />
        <Stat
          label="Resultado do mês"
          value={formatMoney(report.result_cents)}
          tone={report.result_cents >= 0 ? "good" : "bad"}
          hint="Líquido − despesas pagas"
        />
        <Stat label="Ocupação" value={`${Math.round(kpis.occupancy * 100)}%`} hint={`${kpis.soldRoomNights} de ${kpis.availableRoomNights} quartos-noite`} />
        <Stat label="Diária média (ADR)" value={formatMoney(kpis.adrCents)} hint="Receita ÷ quartos-noite vendidos" />
        <Stat label="RevPAR" value={formatMoney(kpis.revparCents)} hint="Receita ÷ quartos-noite disponíveis" />
        <Stat label="A receber" value={formatMoney(report.receivables_cents)} hint="Saldo das contas em aberto" />
      </div>

      <Card className="mt-6">
        <CardHeader title="Movimento diário" description="Recebimentos (verde) e despesas (vermelho) por dia." />
        <div className="overflow-x-auto px-5 py-4">
          <div className="flex h-40 min-w-[40rem] items-end gap-1" role="img" aria-label="Gráfico de recebimentos e despesas por dia">
            {report.daily.map((day) => (
              <div key={day.date} className="group relative flex flex-1 flex-col items-center gap-1">
                <div className="flex h-32 w-full items-end justify-center gap-px">
                  <div className="w-1/2 rounded-t bg-brand-500" style={{ height: `${(day.gross_cents / maxDaily) * 100}%` }} />
                  <div className="w-1/2 rounded-t bg-red-400" style={{ height: `${(day.expenses_cents / maxDaily) * 100}%` }} />
                </div>
                <span className="text-[10px] text-ink-500">{formatDayMonth(day.date).slice(0, 2)}</span>
                <span className="pointer-events-none absolute bottom-full z-10 mb-1 hidden rounded bg-ink-900 px-2 py-1 text-xs whitespace-nowrap text-white group-hover:block">
                  {formatDayMonth(day.date)}: {formatMoney(day.gross_cents)} recebido · {formatMoney(day.expenses_cents)} gasto
                </span>
              </div>
            ))}
          </div>
        </div>
      </Card>

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader title="Por forma de pagamento" />
          {report.by_method.length === 0 ? (
            <EmptyState title="Nenhum recebimento no período" />
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>Forma</Th>
                  <Th align="right">Recebido</Th>
                  <Th align="right">Taxas</Th>
                  <Th align="right">Líquido</Th>
                </tr>
              </thead>
              <tbody>
                {report.by_method.map((row) => (
                  <tr key={row.method}>
                    <Td>
                      {label(METHOD_LABELS, row.method)} <span className="text-xs text-ink-500">({row.count})</span>
                    </Td>
                    <Td align="right">{formatMoney(row.gross_cents)}</Td>
                    <Td align="right">{formatMoney(row.fees_cents)}</Td>
                    <Td align="right">{formatMoney(row.net_cents)}</Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Card>

        <Card>
          <CardHeader title="Por canal de venda" />
          {report.by_source.length === 0 ? (
            <EmptyState title="Nenhum recebimento no período" />
          ) : (
            <Table>
              <thead>
                <tr>
                  <Th>Canal</Th>
                  <Th align="right">Recebido</Th>
                  <Th align="right">Comissões / taxas</Th>
                  <Th align="right">Líquido</Th>
                </tr>
              </thead>
              <tbody>
                {report.by_source.map((row) => (
                  <tr key={row.source}>
                    <Td>{label(SOURCE_LABELS, row.source)}</Td>
                    <Td align="right">{formatMoney(row.gross_cents)}</Td>
                    <Td align="right">{formatMoney(row.fees_cents)}</Td>
                    <Td align="right">{formatMoney(row.net_cents)}</Td>
                  </tr>
                ))}
              </tbody>
            </Table>
          )}
        </Card>

        <Card>
          <CardHeader title="Despesas por categoria" />
          {report.expenses_by_category.length === 0 ? (
            <EmptyState title="Nenhuma despesa paga no período" />
          ) : (
            <ul className="divide-y divide-ink-100">
              {report.expenses_by_category.map((row) => (
                <li key={row.category} className="flex justify-between px-5 py-2.5 text-sm">
                  <span>{label(EXPENSE_CATEGORY_LABELS, row.category)}</span>
                  <span className="tabular">{formatMoney(row.amount_cents)}</span>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card>
          <CardHeader title="Caixa em espécie" description="Dinheiro físico na gaveta." />
          <dl className="divide-y divide-ink-100 text-sm">
            {[
              ["Saldo inicial do mês", report.cash.opening_cents],
              ["Entradas", report.cash.in_cents],
              ["Saídas (despesas e sangrias)", report.cash.out_cents],
              ["Saldo atual", report.cash.closing_cents],
            ].map(([text, value]) => (
              <div key={String(text)} className="flex justify-between px-5 py-2.5">
                <dt className="text-ink-600">{text}</dt>
                <dd className="tabular font-medium">{formatMoney(Number(value))}</dd>
              </div>
            ))}
          </dl>
        </Card>
      </div>
    </>
  );
}
