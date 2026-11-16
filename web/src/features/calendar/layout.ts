import { nightsBetween } from "@/lib/format";

// Cálculo da posição das barras no calendário de ocupação ("tape chart").
// É lógica pura, separada do componente visual — por isso fácil de testar.

export type Span = {
  /** Coluna inicial da barra (1 = primeiro dia visível). */
  column: number;
  /** Quantas colunas (noites) a barra ocupa na janela visível. */
  length: number;
  /** A estadia começou antes do primeiro dia visível. */
  startsBefore: boolean;
  /** A estadia continua depois do último dia visível. */
  endsAfter: boolean;
};

/**
 * Posiciona uma estadia [checkIn, checkOut) em uma janela de `days` dias que
 * começa em `from`. Devolve null se a estadia não aparece na janela.
 *
 * A barra cobre as NOITES da estadia: quem entra dia 10 e sai dia 12 ocupa as
 * colunas dos dias 10 e 11 — a do dia 12 fica livre para o próximo hóspede.
 */
export function placeStay(checkIn: string, checkOut: string, from: string, days: number): Span | null {
  const start = nightsBetween(from, checkIn);
  const end = nightsBetween(from, checkOut);
  if (end <= 0 || start >= days) return null;

  const visibleStart = Math.max(start, 0);
  const visibleEnd = Math.min(end, days);
  return {
    column: visibleStart + 1,
    length: visibleEnd - visibleStart,
    startsBefore: start < 0,
    endsAfter: end > days,
  };
}
