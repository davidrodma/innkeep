// Funções puras de formatação e de datas. Não dependem de servidor nem de
// navegador, então são usadas em qualquer lugar e fáceis de testar.

const TIME_ZONE = "America/Sao_Paulo";

/** Formata centavos como moeda brasileira: 15000 → "R$ 150,00". */
export function formatMoney(cents: number, currency = "BRL"): string {
  return new Intl.NumberFormat("pt-BR", { style: "currency", currency }).format(cents / 100);
}

/**
 * Converte o texto digitado em um campo de valor para centavos.
 * Aceita "150", "150,5", "1.234,56" e "R$ 80,00". Devolve null se inválido.
 *
 * A conta é feita com inteiros (reais e centavos separados) para não cair nas
 * imprecisões de ponto flutuante: em JavaScript, 19.99 * 100 dá 1998.9999...
 */
export function parseMoney(input: string): number | null {
  const cleaned = input.replace(/[R$\s]/g, "").replace(/\./g, "").replace(",", ".");
  if (!/^\d+(\.\d{1,2})?$/.test(cleaned)) return null;
  const [whole = "0", fraction = ""] = cleaned.split(".");
  return Number(whole) * 100 + Number(fraction.padEnd(2, "0"));
}

/** "2026-11-14" → "14/11/2026". */
export function formatDate(iso: string): string {
  if (!/^\d{4}-\d{2}-\d{2}/.test(iso)) return iso;
  const [year, month, day] = iso.slice(0, 10).split("-");
  return `${day}/${month}/${year}`;
}

/** "2026-11-14" → "14/11" (para cabeçalhos de calendário). */
export function formatDayMonth(iso: string): string {
  return `${iso.slice(8, 10)}/${iso.slice(5, 7)}`;
}

/** Instante ISO → "14/11/2026 15:30" no horário de Brasília. */
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "";
  return new Intl.DateTimeFormat("pt-BR", {
    timeZone: TIME_ZONE,
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

/** Data de hoje ("AAAA-MM-DD") no fuso de Brasília — não no fuso do servidor. */
export function today(): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone: TIME_ZONE }).format(new Date());
}

/**
 * Soma dias a uma data "AAAA-MM-DD". A conta é feita em UTC de propósito:
 * assim nenhum horário de verão desloca o resultado em um dia.
 */
export function addDays(iso: string, days: number): string {
  const date = new Date(`${iso}T00:00:00Z`);
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

/** Número de noites entre duas datas "AAAA-MM-DD". */
export function nightsBetween(checkIn: string, checkOut: string): number {
  const start = Date.parse(`${checkIn}T00:00:00Z`);
  const end = Date.parse(`${checkOut}T00:00:00Z`);
  return Math.round((end - start) / 86_400_000);
}

/** Lista as datas de `from` (inclusive) até `to` (exclusive). */
export function eachDay(from: string, to: string): string[] {
  const days: string[] = [];
  for (let day = from; day < to; day = addDays(day, 1)) days.push(day);
  return days;
}

const WEEKDAYS = ["dom", "seg", "ter", "qua", "qui", "sex", "sáb"];

/** Dia da semana abreviado de uma data "AAAA-MM-DD". */
export function weekday(iso: string): string {
  return WEEKDAYS[new Date(`${iso}T00:00:00Z`).getUTCDay()] ?? "";
}

export function isWeekend(iso: string): boolean {
  const day = new Date(`${iso}T00:00:00Z`).getUTCDay();
  return day === 0 || day === 6;
}

/** Primeiro dia do mês de uma data: "2026-11-14" → "2026-11-01". */
export function startOfMonth(iso: string): string {
  return `${iso.slice(0, 7)}-01`;
}

/** Primeiro dia do mês seguinte. */
export function startOfNextMonth(iso: string): string {
  const date = new Date(`${iso.slice(0, 7)}-01T00:00:00Z`);
  date.setUTCMonth(date.getUTCMonth() + 1);
  return date.toISOString().slice(0, 10);
}

/** Pontos-base → porcentagem legível: 1500 → "15%". */
export function formatBps(bps: number): string {
  return `${(bps / 100).toLocaleString("pt-BR", { maximumFractionDigits: 2 })}%`;
}
