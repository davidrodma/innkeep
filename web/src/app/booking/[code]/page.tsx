import type { Metadata } from "next";
import { cookies } from "next/headers";
import Link from "next/link";

import { SiteFooter, SiteHeader } from "@/components/public/site-chrome";
import { AutoRefresh } from "@/components/ui/auto-refresh";
import { StatusBadge } from "@/components/ui/layout";
import { LookupForm } from "@/features/booking/lookup-form";
import { publishedProperties } from "@/features/booking/queries";
import { api } from "@/lib/api";
import { formatDate, formatDateTime, formatMoney } from "@/lib/format";
import { REJECT_LABELS, label } from "@/lib/labels";
import { BOOKING_COOKIE } from "@/lib/session-cookies";
import type { PublicReservation } from "@/lib/types";

export const metadata: Metadata = { title: "Sua reserva" };

type Props = { params: Promise<{ code: string }> };

export default async function BookingStatusPage({ params }: Props) {
  const code = decodeURIComponent((await params).code).toUpperCase();

  // O e-mail vem do cookie gravado quando a reserva foi feita (ou consultada)
  // neste navegador. Sem ele, pedimos o e-mail: o localizador sozinho não basta.
  let saved: Record<string, string> = {};
  try {
    saved = JSON.parse((await cookies()).get(BOOKING_COOKIE)?.value ?? "{}") as Record<string, string>;
  } catch {
    saved = {};
  }
  const email = saved[code];
  const reservation = email
    ? await api<PublicReservation>("/v1/public/reservations/lookup", { method: "POST", anonymous: true, body: { code, email } }).catch(() => null)
    : null;

  if (!reservation) {
    return (
      <>
        <SiteHeader />
        <main className="mx-auto max-w-sm px-4 py-12">
          <h1 className="text-2xl font-semibold tracking-tight text-ink-900">Reserva {code}</h1>
          <p className="mt-1 text-sm text-ink-600">Confirme o e-mail usado na reserva para ver os detalhes.</p>
          <div className="mt-6 rounded-xl border border-ink-200 bg-white p-6">
            <LookupForm code={code} />
          </div>
        </main>
        <SiteFooter />
      </>
    );
  }

  const property = (await publishedProperties().catch(() => [])).find((item) => item.id === reservation.property_id);
  const processing = reservation.status === "REQUESTED";
  const awaitingDeposit = reservation.status === "PENDING" && reservation.deposit_due_cents > 0;

  return (
    <>
      {/* Enquanto a saga não termina, a página se atualiza sozinha. */}
      {processing ? <AutoRefresh /> : null}
      <SiteHeader title={property?.name} href={property ? `/p/${property.slug}` : "/"} />
      <main className="mx-auto max-w-2xl px-4 py-10">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="font-mono text-sm text-ink-500">{reservation.code}</p>
            <h1 className="text-2xl font-semibold tracking-tight text-ink-900">
              {processing
                ? "Estamos confirmando a disponibilidade…"
                : reservation.status === "REJECTED"
                  ? "Não foi possível concluir a reserva"
                  : awaitingDeposit
                    ? "Quase lá: falta o sinal"
                    : reservation.status === "CONFIRMED"
                      ? "Reserva confirmada!"
                      : "Sua reserva"}
            </h1>
          </div>
          <StatusBadge status={reservation.status} />
        </div>

        {reservation.status === "REJECTED" ? (
          <p className="mt-6 rounded-xl bg-red-50 px-5 py-4 text-sm text-red-800">
            {label(REJECT_LABELS, reservation.reject_reason)}{" "}
            {property ? (
              <Link href={`/p/${property.slug}`} className="underline">
                Escolher outras datas
              </Link>
            ) : null}
          </p>
        ) : null}

        {awaitingDeposit ? (
          <section className="mt-6 rounded-xl border border-amber-300 bg-amber-50 p-5 text-sm text-amber-900">
            <h2 className="text-base font-semibold">Pague o sinal para garantir a reserva</h2>
            <p className="tabular mt-2 text-2xl font-semibold">{formatMoney(reservation.deposit_due_cents)}</p>
            {property?.pix_key ? (
              <p className="mt-2">
                Chave Pix: <strong className="select-all">{property.pix_key}</strong>
              </p>
            ) : null}
            {reservation.hold_expires_at ? (
              <p className="mt-2">
                Prazo: até <strong>{formatDateTime(reservation.hold_expires_at)}</strong> (horário de Brasília). Depois disso a reserva
                expira.
              </p>
            ) : null}
            <p className="mt-2">Envie o comprovante para a propriedade{property?.phone ? ` (${property.phone})` : ""}. A confirmação chega por e-mail.</p>
          </section>
        ) : null}

        <section className="mt-6 rounded-xl border border-ink-200 bg-white p-5 text-sm">
          <dl className="grid grid-cols-2 gap-4">
            <div>
              <dt className="text-ink-500">Titular</dt>
              <dd className="font-medium text-ink-900">{reservation.holder_name}</dd>
            </div>
            <div>
              <dt className="text-ink-500">Hóspedes</dt>
              <dd>
                {reservation.adults} adulto(s){reservation.children ? `, ${reservation.children} criança(s)` : ""}
              </dd>
            </div>
            <div>
              <dt className="text-ink-500">Entrada</dt>
              <dd>
                {formatDate(reservation.check_in)}
                {property ? `, a partir das ${property.check_in_time}` : ""}
              </dd>
            </div>
            <div>
              <dt className="text-ink-500">Saída</dt>
              <dd>
                {formatDate(reservation.check_out)}
                {property ? `, até as ${property.check_out_time}` : ""}
              </dd>
            </div>
            <div className="col-span-2 flex justify-between border-t border-ink-200 pt-4 text-base">
              <dt className="font-semibold text-ink-900">Total ({reservation.nights} noite(s))</dt>
              <dd className="tabular font-semibold text-ink-900">{formatMoney(reservation.total_cents)}</dd>
            </div>
          </dl>
        </section>

        {property ? (
          <p className="mt-6 text-sm text-ink-600">
            Dúvidas? Fale com {property.name}
            {property.phone ? ` pelo telefone ${property.phone}` : ""}
            {property.email ? ` ou pelo e-mail ${property.email}` : ""}.
          </p>
        ) : null}
      </main>
      <SiteFooter />
    </>
  );
}
