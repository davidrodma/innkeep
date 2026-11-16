import type { AvailabilityGrid, Reservation } from "@/lib/types";

// Indicadores clássicos da hotelaria, calculados a partir dos dados das APIs.
// Funções puras (sem I/O), testadas em kpis.test.ts.

export type Kpis = {
  /** Quartos-noite disponíveis no período (capacidade). */
  availableRoomNights: number;
  /** Quartos-noite ocupados por reservas. */
  soldRoomNights: number;
  /** Taxa de ocupação, de 0 a 1. */
  occupancy: number;
  /** Receita de hospedagem do período, em centavos (regime de competência). */
  roomRevenueCents: number;
  /** ADR (Average Daily Rate): diária média vendida = receita / quartos-noite vendidos. */
  adrCents: number;
  /** RevPAR (Revenue per Available Room): receita / quartos-noite disponíveis. */
  revparCents: number;
};

const OCCUPYING = new Set(["CONFIRMED", "CHECKED_IN", "CHECKED_OUT"]);

/**
 * Calcula os indicadores do período [from, to).
 *
 * A receita aqui é por COMPETÊNCIA: conta a diária da noite em que o hóspede
 * dormiu, não o dia em que pagou. Por isso difere do "recebido" do relatório
 * financeiro, que é por regime de CAIXA. As duas visões são úteis: uma mostra o
 * desempenho comercial, a outra o dinheiro que efetivamente entrou.
 */
export function computeKpis(grid: AvailabilityGrid, reservations: Reservation[], from: string, to: string): Kpis {
  let availableRoomNights = 0;
  for (const roomType of grid.room_types) {
    for (const day of roomType.days) availableRoomNights += day.total;
  }

  let soldRoomNights = 0;
  let roomRevenueCents = 0;
  for (const reservation of reservations) {
    if (!OCCUPYING.has(reservation.status)) continue;
    for (const night of reservation.nightly_rates) {
      if (night.date >= from && night.date < to) {
        soldRoomNights += 1;
        roomRevenueCents += night.price_cents;
      }
    }
  }

  return {
    availableRoomNights,
    soldRoomNights,
    occupancy: availableRoomNights > 0 ? soldRoomNights / availableRoomNights : 0,
    roomRevenueCents,
    adrCents: soldRoomNights > 0 ? Math.round(roomRevenueCents / soldRoomNights) : 0,
    revparCents: availableRoomNights > 0 ? Math.round(roomRevenueCents / availableRoomNights) : 0,
  };
}
