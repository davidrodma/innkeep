import { startOfMonth, startOfNextMonth, today } from "@/lib/format";

const MONTHS = ["janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"];

/** Período [from, to) do mês pedido na URL (?month=AAAA-MM) ou do mês atual. */
export function monthPeriod(month?: string) {
  const base = /^\d{4}-\d{2}$/.test(month ?? "") ? `${month}-01` : startOfMonth(today());
  const from = startOfMonth(base);
  const to = startOfNextMonth(base);
  const previous = startOfMonth(new Date(Date.parse(`${from}T00:00:00Z`) - 86_400_000).toISOString().slice(0, 10));
  return {
    from,
    to,
    label: `${MONTHS[Number(from.slice(5, 7)) - 1]} de ${from.slice(0, 4)}`,
    previousMonth: previous.slice(0, 7),
    nextMonth: to.slice(0, 7),
  };
}
