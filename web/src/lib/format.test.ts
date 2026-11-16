import { describe, expect, it } from "vitest";

import { addDays, eachDay, formatDate, formatMoney, nightsBetween, parseMoney, startOfNextMonth } from "./format";

describe("formatMoney", () => {
  it("formata centavos como reais", () => {
    // O Intl usa um espaço não separável entre "R$" e o número.
    expect(formatMoney(15000).replace(/\s/g, " ")).toBe("R$ 150,00");
    expect(formatMoney(123456).replace(/\s/g, " ")).toBe("R$ 1.234,56");
  });
});

describe("parseMoney", () => {
  it.each([
    ["150", 15000],
    ["150,5", 15050],
    ["1.234,56", 123456],
    ["R$ 80,00", 8000],
    ["0,01", 1],
    ["19,99", 1999],
  ])("converte %s em %d centavos", (input, cents) => {
    expect(parseMoney(input)).toBe(cents);
  });

  it("não sofre com imprecisão de ponto flutuante", () => {
    // Em JavaScript, 19.99 * 100 = 1998.9999999999998 — por isso a conversão usa inteiros.
    expect(parseMoney("19,99")).toBe(1999);
    expect(parseMoney("0,29")).toBe(29);
  });

  it.each(["", "abc", "1,234", "-5", "10,999"])("rejeita %s", (input) => {
    expect(parseMoney(input)).toBeNull();
  });
});

describe("datas de estadia", () => {
  it("soma dias atravessando meses e anos", () => {
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
    expect(addDays("2028-02-28", 1)).toBe("2028-02-29"); // ano bissexto
    expect(addDays("2026-03-01", -1)).toBe("2026-02-28");
  });

  it("conta noites e lista os dias do intervalo semiaberto", () => {
    expect(nightsBetween("2026-11-10", "2026-11-13")).toBe(3);
    expect(eachDay("2026-11-10", "2026-11-13")).toEqual(["2026-11-10", "2026-11-11", "2026-11-12"]);
  });

  it("formata e calcula o início do mês seguinte", () => {
    expect(formatDate("2026-11-14")).toBe("14/11/2026");
    expect(startOfNextMonth("2026-12-15")).toBe("2027-01-01");
  });
});
