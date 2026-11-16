import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { SiteFooter, SiteHeader } from "@/components/public/site-chrome";
import { buttonClass } from "@/components/ui/button";
import { publicProperty, searchOffers } from "@/features/booking/queries";
import { SearchForm } from "@/features/booking/search-form";
import { ApiError } from "@/lib/api";
import { addDays, formatDate, formatMoney, today } from "@/lib/format";
import type { OfferSearch, PublicProperty } from "@/lib/types";

type Props = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ check_in?: string; check_out?: string; adults?: string; children?: string }>;
};

async function load(slug: string): Promise<PublicProperty> {
  try {
    return await publicProperty(slug);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const property = await load((await params).slug);
  return { title: property.name, description: property.description.slice(0, 160) };
}

const REASONS: Record<string, string> = {
  NO_AVAILABILITY: "Esgotado nestas datas",
  OVER_CAPACITY: "Não comporta esse número de hóspedes",
  MIN_STAY: "Exige estadia mínima maior",
  CLOSED: "Indisponível nestas datas",
};

export default async function PropertyPage({ params, searchParams }: Props) {
  const { slug } = await params;
  const property = await load(slug);
  const search = await searchParams;

  const minDate = today();
  const checkIn = search.check_in && search.check_in >= minDate ? search.check_in : "";
  const checkOut = search.check_out && checkIn && search.check_out > checkIn ? search.check_out : "";
  const adults = Math.min(Math.max(Number(search.adults) || 2, 1), 20);
  const children = Math.min(Math.max(Number(search.children) || 0, 0), 20);

  let results: OfferSearch | null = null;
  let searchError = "";
  if (checkIn && checkOut) {
    try {
      results = await searchOffers(property.id, checkIn, checkOut, adults, children);
    } catch (error) {
      searchError = error instanceof ApiError ? error.message : "Não foi possível consultar a disponibilidade.";
    }
  }
  const roomTypes = property.room_types ?? [];
  const deposit = property.deposit_percent;

  return (
    <>
      <SiteHeader title={property.name} href={`/p/${property.slug}`} />
      <main className="mx-auto max-w-5xl px-4 py-10">
        <h1 className="text-3xl font-semibold tracking-tight text-ink-900">{property.name}</h1>
        <p className="mt-1 text-sm text-ink-500">
          {[property.address.line, property.address.district, property.address.city, property.address.state].filter(Boolean).join(", ")}
        </p>
        {property.description ? <p className="mt-4 max-w-3xl text-ink-700">{property.description}</p> : null}

        <div className="mt-8">
          <SearchForm
            slug={property.slug}
            minDate={minDate}
            defaults={{ checkIn: checkIn || minDate, checkOut: checkOut || addDays(checkIn || minDate, 1), adults, children }}
          />
        </div>

        <section id="resultados" className="mt-8 scroll-mt-6">
          {searchError ? <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{searchError}</p> : null}
          {results ? (
            <>
              <h2 className="mb-4 text-lg font-semibold text-ink-900">
                {formatDate(results.check_in)} a {formatDate(results.check_out)} · {results.nights} noite(s)
              </h2>
              <ul className="space-y-4">
                {results.offers.map((offer) => {
                  const roomType = roomTypes.find((type) => type.id === offer.room_type_id);
                  const params = new URLSearchParams({
                    room_type_id: offer.room_type_id,
                    check_in: results.check_in,
                    check_out: results.check_out,
                    adults: String(adults),
                    children: String(children),
                  });
                  return (
                    <li key={offer.room_type_id} className="flex flex-col gap-4 rounded-xl border border-ink-200 bg-white p-5 sm:flex-row sm:items-center sm:justify-between">
                      <div>
                        <p className="text-base font-semibold text-ink-900">{offer.name}</p>
                        <p className="mt-0.5 text-sm text-ink-500">
                          Até {offer.max_occupancy} pessoa(s){roomType?.bed_configuration ? ` · ${roomType.bed_configuration}` : ""}
                        </p>
                        {roomType?.amenities.length ? <p className="mt-2 text-xs text-ink-500">{roomType.amenities.join(" · ")}</p> : null}
                      </div>
                      <div className="flex items-center gap-4 sm:text-right">
                        {offer.bookable ? (
                          <>
                            <div>
                              <p className="tabular text-xl font-semibold text-ink-900">{formatMoney(offer.total_cents)}</p>
                              <p className="text-xs text-ink-500">
                                total · {offer.available === 1 ? "último disponível" : `${offer.available} disponíveis`}
                              </p>
                            </div>
                            <Link href={`/p/${property.slug}/book?${params.toString()}`} className={buttonClass("primary")}>
                              Reservar
                            </Link>
                          </>
                        ) : (
                          <p className="text-sm text-ink-500">{REASONS[offer.reason ?? ""] ?? "Indisponível"}</p>
                        )}
                      </div>
                    </li>
                  );
                })}
              </ul>
            </>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2">
              {roomTypes.map((type) => (
                <div key={type.id} className="rounded-xl border border-ink-200 bg-white p-5">
                  <p className="text-base font-semibold text-ink-900">{type.name}</p>
                  <p className="mt-0.5 text-sm text-ink-500">Até {type.max_occupancy} pessoa(s)</p>
                  {type.description ? <p className="mt-2 text-sm text-ink-600">{type.description}</p> : null}
                  <p className="mt-3 text-sm text-ink-700">
                    A partir de <strong className="tabular">{formatMoney(type.base_price_cents)}</strong> por noite
                  </p>
                </div>
              ))}
            </div>
          )}
        </section>

        <section className="mt-12 grid gap-6 text-sm text-ink-600 sm:grid-cols-3">
          <div>
            <h2 className="mb-1 font-semibold text-ink-900">Horários</h2>
            <p>
              Entrada a partir das {property.check_in_time}. Saída até as {property.check_out_time}.
            </p>
          </div>
          <div>
            <h2 className="mb-1 font-semibold text-ink-900">Garantia</h2>
            <p>
              {deposit > 0
                ? `A reserva é confirmada com o pagamento de ${deposit}% do valor como sinal${
                    property.hold_minutes ? `, em até ${Math.round(property.hold_minutes / 60)} hora(s)` : ""
                  }. O restante é pago na estadia.`
                : "Não é preciso pagar sinal: o pagamento é feito na estadia."}
            </p>
          </div>
          <div>
            <h2 className="mb-1 font-semibold text-ink-900">Cancelamento</h2>
            <p>{property.cancellation_policy || "Consulte a propriedade."}</p>
          </div>
        </section>
      </main>
      <SiteFooter />
    </>
  );
}
