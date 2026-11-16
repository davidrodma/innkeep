import { describe, expect, it } from "vitest";

import { placeStay } from "./layout";

// Janela de 7 dias começando em 10/11.
const FROM = "2026-11-10";
const DAYS = 7;

describe("placeStay", () => {
  it("posiciona uma estadia inteira dentro da janela", () => {
    // Entrada dia 11, saída dia 13: ocupa as noites 11 e 12 (colunas 2 e 3).
    expect(placeStay("2026-11-11", "2026-11-13", FROM, DAYS)).toEqual({
      column: 2,
      length: 2,
      startsBefore: false,
      endsAfter: false,
    });
  });

  it("corta a barra de uma estadia que começou antes da janela", () => {
    expect(placeStay("2026-11-08", "2026-11-12", FROM, DAYS)).toEqual({
      column: 1,
      length: 2,
      startsBefore: true,
      endsAfter: false,
    });
  });

  it("corta a barra de uma estadia que termina depois da janela", () => {
    expect(placeStay("2026-11-15", "2026-11-20", FROM, DAYS)).toEqual({
      column: 6,
      length: 2,
      startsBefore: false,
      endsAfter: true,
    });
  });

  it("ignora estadias fora da janela (o dia da saída não conta)", () => {
    expect(placeStay("2026-11-05", "2026-11-10", FROM, DAYS)).toBeNull();
    expect(placeStay("2026-11-17", "2026-11-18", FROM, DAYS)).toBeNull();
  });
});
