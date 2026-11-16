import { describe, expect, it } from "vitest";

import type { AvailabilityGrid, Reservation } from "@/lib/types";

import { computeKpis } from "./kpis";

function grid(roomsPerDay: number[]): AvailabilityGrid {
  return {
    from: "2026-11-01",
    to: "2026-11-04",
    room_types: [
      {
        room_type_id: "t1",
        name: "Quarto",
        max_occupancy: 2,
        base_price_cents: 10000,
        currency: "BRL",
        days: roomsPerDay.map((total, index) => ({
          date: `2026-11-0${index + 1}`,
          total,
          available: total,
          price_cents: 10000,
          min_stay: 1,
          closed: false,
        })),
      },
    ],
  };
}

function reservation(status: Reservation["status"], nights: [string, number][]): Reservation {
  return { status, nightly_rates: nights.map(([date, price_cents]) => ({ date, price_cents })) } as Reservation;
}

describe("computeKpis", () => {
  it("calcula ocupação, ADR e RevPAR", () => {
    // 3 dias x 2 quartos = 6 quartos-noite disponíveis.
    const kpis = computeKpis(
      grid([2, 2, 2]),
      [
        reservation("CONFIRMED", [
          ["2026-11-01", 10000],
          ["2026-11-02", 20000],
        ]),
        reservation("CHECKED_OUT", [["2026-11-03", 15000]]),
      ],
      "2026-11-01",
      "2026-11-04",
    );
    expect(kpis.availableRoomNights).toBe(6);
    expect(kpis.soldRoomNights).toBe(3);
    expect(kpis.occupancy).toBeCloseTo(0.5);
    expect(kpis.roomRevenueCents).toBe(45000);
    expect(kpis.adrCents).toBe(15000); // 450 / 3 noites
    expect(kpis.revparCents).toBe(7500); // 450 / 6 quartos-noite
  });

  it("ignora reservas canceladas e noites fora do período", () => {
    const kpis = computeKpis(
      grid([1, 1, 1]),
      [
        reservation("CANCELLED", [["2026-11-01", 10000]]),
        reservation("CONFIRMED", [
          ["2026-10-31", 10000],
          ["2026-11-01", 10000],
        ]),
      ],
      "2026-11-01",
      "2026-11-04",
    );
    expect(kpis.soldRoomNights).toBe(1);
    expect(kpis.roomRevenueCents).toBe(10000);
  });

  it("não divide por zero sem capacidade nem vendas", () => {
    const kpis = computeKpis(grid([]), [], "2026-11-01", "2026-11-04");
    expect(kpis).toMatchObject({ occupancy: 0, adrCents: 0, revparCents: 0 });
  });
});
