import type { Metadata } from "next";

import { MonthNav } from "@/components/admin/month-nav";
import { Badge, Card, CardHeader, EmptyState, Table, Td, Th } from "@/components/ui/layout";
import { deleteExpense } from "@/features/finance/actions";
import { ExpenseForm, PayExpenseForm } from "@/features/finance/forms";
import { monthPeriod } from "@/features/finance/period";
import { api, query } from "@/lib/api";
import { formatDate, formatMoney, today } from "@/lib/format";
import { EXPENSE_CATEGORY_LABELS, METHOD_LABELS, label } from "@/lib/labels";
import { currentProperty } from "@/lib/session";
import type { Expense, List } from "@/lib/types";

export const metadata: Metadata = { title: "Despesas" };

type Props = { searchParams: Promise<{ month?: string }> };

export default async function ExpensesPage({ searchParams }: Props) {
  const property = await currentProperty();
  const period = monthPeriod((await searchParams).month);
  const expenses = (await api<List<Expense>>(`/v1/expenses${query({ property_id: property.id, from: period.from, to: period.to })}`)).items;
  const total = expenses.reduce((sum, expense) => sum + expense.amount_cents, 0);
  const pending = expenses.filter((expense) => expense.status === "PENDING").reduce((sum, expense) => sum + expense.amount_cents, 0);
  const currentDay = today();

  return (
    <>
      <MonthNav path="/admin/finance/expenses" label={period.label} previousMonth={period.previousMonth} nextMonth={period.nextMonth} />
      <Card className="mb-6">
        <CardHeader title="Registrar despesa" />
        <div className="px-5 py-4">
          <ExpenseForm propertyId={property.id} today={currentDay} />
        </div>
      </Card>

      <Card>
        <CardHeader title="Despesas do mês" description={`Total ${formatMoney(total)} · a pagar ${formatMoney(pending)}`} />
        {expenses.length === 0 ? (
          <EmptyState title="Nenhuma despesa neste mês" />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Data</Th>
                <Th>Descrição</Th>
                <Th>Categoria</Th>
                <Th align="right">Valor</Th>
                <Th>Situação</Th>
                <Th />
              </tr>
            </thead>
            <tbody>
              {expenses.map((expense) => (
                <tr key={expense.id}>
                  <Td>{formatDate(expense.due_date)}</Td>
                  <Td>
                    <span className="font-medium text-ink-900">{expense.description}</span>
                    {expense.supplier ? <span className="block text-xs text-ink-500">{expense.supplier}</span> : null}
                  </Td>
                  <Td>{label(EXPENSE_CATEGORY_LABELS, expense.category)}</Td>
                  <Td align="right">{formatMoney(expense.amount_cents)}</Td>
                  <Td>
                    {expense.status === "PAID" ? (
                      <Badge tone="brand">Paga · {label(METHOD_LABELS, expense.method)}</Badge>
                    ) : (
                      <Badge tone={expense.due_date < currentDay ? "red" : "amber"}>
                        {expense.due_date < currentDay ? "Vencida" : "A pagar"}
                      </Badge>
                    )}
                  </Td>
                  <Td align="right">
                    {expense.status === "PENDING" ? (
                      <div className="flex items-center justify-end gap-2">
                        <PayExpenseForm id={expense.id} today={currentDay} />
                        <form action={deleteExpense}>
                          <input type="hidden" name="id" value={expense.id} />
                          <button type="submit" className="text-xs text-ink-500 hover:text-red-700">
                            Excluir
                          </button>
                        </form>
                      </div>
                    ) : null}
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
