import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { SiteFooter, SiteHeader } from "@/components/public/site-chrome";
import { BookingForm } from "@/features/booking/booking-form";
import { publicProperty, searchOffers } from "@/features/booking/queries";
import { ApiError } from "@/lib/api";
import { formatDate, formatMoney } from "@/lib/format";

export const metadata: Metadata = { title: "Finalizar reserva" };

type Props = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ room_type_id?: string; check_in?: string; check_out?: string; adults?: string; children?: string }>;
};

export default async function BookPage({ params, searchParams }: Props) {
  const { slug } = await params;
  const search = await searchParams;
  let property;
  try {
    property = await publicProperty(slug);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) notFound();
    throw error;
  }
  const adults = Math.max(Number(search.adults) || 1, 1);
  const children = Math.max(Number(search.children) || 0, 0);
  if (!search.room_type_id || !search.check_in || !search.check_out) notFound();

  // O preço exibido vem de uma nova consulta (não da URL): o visitante não
  // consegue alterar o valor editando o endereço. E, de toda forma, quem fixa
  // o preço da reserva é o inventário, no back-end.
  const results = await searchOffers(property.id, search.check_in, search.check_out, adults, children).catch(() => null);
  const offer = results?.offers.find((item) => item.room_type_id === search.room_type_id);
  const back = `/p/${slug}?check_in=${search.check_in}&check_out=${search.check_out}&adults=${adults}&children=${children}#resultados`;
  const deposit = offer ? Math.round((offer.total_cents * property.deposit_percent) / 100) : 0;

  return (
    <>
      <SiteHeader title={property.name} href={`/p/${slug}`} />
      <main className="mx-auto max-w-5xl px-4 py-10">
        <Link href={back} className="text-sm text-brand-700 hover:underline">
          ← Voltar aos resultados
        </Link>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight text-ink-900">Finalizar reserva</h1>

        {!offer || !offer.bookable ? (
          <p className="mt-6 rounded-xl border border-ink-200 bg-white px-6 py-8 text-center text-sm text-ink-600">
            Esta acomodação não está mais disponível para as datas escolhidas.{" "}
            <Link href={back} className="text-brand-700 underline">
              Escolher outra opção
            </Link>
          </p>
        ) : (
          <div className="mt-6 grid gap-6 lg:grid-cols-[1fr_20rem]">
            <section className="rounded-xl border border-ink-200 bg-white p-5 sm:p-6">
              <h2 className="mb-4 text-base font-semibold text-ink-900">Seus dados</h2>
              <BookingForm
                propertyId={property.id}
                roomTypeId={offer.room_type_id}
                checkIn={search.check_in}
                checkOut={search.check_out}
                adults={adults}
                childCount={children}
                cancellationPolicy={property.cancellation_policy}
                idempotencyKey={crypto.randomUUID()}
              />
            </section>
            <aside className="h-fit rounded-xl border border-ink-200 bg-white p-5 text-sm">
              <h2 className="text-base font-semibold text-ink-900">{offer.name}</h2>
              <dl className="mt-3 space-y-2">
                <div className="flex justify-between">
                  <dt className="text-ink-500">Entrada</dt>
                  <dd>{formatDate(search.check_in)}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-500">Saída</dt>
                  <dd>{formatDate(search.check_out)}</dd>
                </div>
                <div className="flex justify-between">
                  <dt className="text-ink-500">Hóspedes</dt>
                  <dd>
                    {adults} adulto(s){children ? `, ${children} criança(s)` : ""}
                  </dd>
                </div>
              </dl>
              <ul className="mt-4 space-y-1 border-t border-ink-200 pt-4 text-ink-600">
                {offer.nightly_rates.map((night) => (
                  <li key={night.date} className="flex justify-between">
                    <span>{formatDate(night.date)}</span>
                    <span className="tabular">{formatMoney(night.price_cents)}</span>
                  </li>
                ))}
              </ul>
              <div className="mt-3 flex justify-between border-t border-ink-200 pt-3 text-base font-semibold text-ink-900">
                <span>Total</span>
                <span className="tabular">{formatMoney(offer.total_cents)}</span>
              </div>
              {deposit > 0 ? (
                <p className="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-800">
                  Para garantir a reserva, pague o sinal de {formatMoney(deposit)} ({property.deposit_percent}%) após confirmar. As
                  instruções aparecem na próxima tela.
                </p>
              ) : null}
            </aside>
          </div>
        )}
      </main>
      <SiteFooter />
    </>
  );
}
