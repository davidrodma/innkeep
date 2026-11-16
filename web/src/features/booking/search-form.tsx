"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/form";
import { addDays } from "@/lib/format";

/**
 * Busca de disponibilidade. Os parâmetros vão para a URL (?check_in=...), e a
 * página (Server Component) consulta a API com eles — a busca fica
 * compartilhável e o botão "voltar" do navegador funciona.
 */
export function SearchForm({
  slug,
  minDate,
  defaults,
}: {
  slug: string;
  minDate: string;
  defaults: { checkIn: string; checkOut: string; adults: number; children: number };
}) {
  const router = useRouter();
  const [checkIn, setCheckIn] = useState(defaults.checkIn);
  const [checkOut, setCheckOut] = useState(defaults.checkOut);
  const [adults, setAdults] = useState(defaults.adults);
  const [children, setChildren] = useState(defaults.children);

  return (
    <form
      className="grid gap-3 rounded-xl border border-ink-200 bg-white p-4 sm:grid-cols-[1fr_1fr_7rem_7rem_auto] sm:items-end"
      onSubmit={(event) => {
        event.preventDefault();
        const params = new URLSearchParams({ check_in: checkIn, check_out: checkOut, adults: String(adults), children: String(children) });
        router.push(`/p/${slug}?${params.toString()}#resultados`);
      }}
    >
      <div>
        <label htmlFor="check_in" className="mb-1 block text-sm font-medium text-ink-700">
          Entrada
        </label>
        <Input
          name="check_in"
          type="date"
          min={minDate}
          value={checkIn}
          onChange={(event) => {
            setCheckIn(event.target.value);
            if (event.target.value >= checkOut) setCheckOut(addDays(event.target.value, 1));
          }}
          required
        />
      </div>
      <div>
        <label htmlFor="check_out" className="mb-1 block text-sm font-medium text-ink-700">
          Saída
        </label>
        <Input name="check_out" type="date" min={addDays(checkIn, 1)} value={checkOut} onChange={(event) => setCheckOut(event.target.value)} required />
      </div>
      <div>
        <label htmlFor="adults" className="mb-1 block text-sm font-medium text-ink-700">
          Adultos
        </label>
        <Select name="adults" value={adults} onChange={(event) => setAdults(Number(event.target.value))}>
          {Array.from({ length: 10 }, (_, index) => index + 1).map((value) => (
            <option key={value} value={value}>
              {value}
            </option>
          ))}
        </Select>
      </div>
      <div>
        <label htmlFor="children" className="mb-1 block text-sm font-medium text-ink-700">
          Crianças
        </label>
        <Select name="children" value={children} onChange={(event) => setChildren(Number(event.target.value))}>
          {Array.from({ length: 7 }, (_, index) => index).map((value) => (
            <option key={value} value={value}>
              {value}
            </option>
          ))}
        </Select>
      </div>
      <Button type="submit" className="w-full sm:w-auto">
        Ver disponibilidade
      </Button>
    </form>
  );
}
