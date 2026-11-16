import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { AutoRefresh } from "@/components/ui/auto-refresh";
import { Badge, Card, CardHeader, PageHeader, StatusBadge, Table, Td, Th } from "@/components/ui/layout";
import { ChargeForm, PaymentForm, VoidEntryForm } from "@/features/reservations/folio-forms";
import {
  getFolioByReservation,
  getHistory,
  getRegistration,
  getReservation,
  listRooms,
} from "@/features/reservations/queries";
import { RegistrationForm } from "@/features/reservations/registration-form";
import { ReservationActions } from "@/features/reservations/reservation-actions";
import { ApiError } from "@/lib/api";
import { formatDate, formatDateTime, formatMoney } from "@/lib/format";
import { DOCUMENT_LABELS, EVENT_LABELS, METHOD_LABELS, REJECT_LABELS, SOURCE_LABELS, label } from "@/lib/labels";
import type { Folio, Reservation } from "@/lib/types";

export const metadata: Metadata = { title: "Reserva" };

type Props = { params: Promise<{ id: string }> };

export default async function ReservationPage({ params }: Props) {
  const { id } = await params;

  let reservation: Reservation;
  try {
    reservation = await getReservation(id);
  } catch (error) {
    if (error instanceof ApiError && (error.status === 404 || error.status === 400)) notFound();
    throw error;
  }

  // Cada bloco da tela vem de um serviço diferente; as consultas saem em paralelo.
  const [folio, registration, history, rooms] = await Promise.all([
    getFolioByReservation(id),
    getRegistration(id),
    getHistory(id),
    listRooms(reservation.property_id),
  ]);

  // Enquanto a saga está em andamento, a tela se atualiza sozinha.
  const settling = reservation.status === "REQUESTED" || reservation.pending_change !== null;
  const waitingFolio = !folio && !["REQUESTED", "REJECTED"].includes(reservation.status);

  return (
    <>
      {settling || waitingFolio ? <AutoRefresh /> : null}

      <PageHeader
        title={reservation.holder.name}
        description={`Reserva ${reservation.code} · ${label(SOURCE_LABELS, reservation.source)}${
          reservation.external_ref ? ` nº ${reservation.external_ref}` : ""
        }`}
        action={<StatusBadge status={reservation.status} />}
      />

      {reservation.status === "REQUESTED" ? (
        <Notice tone="neutral">Verificando a disponibilidade e reservando o quarto…</Notice>
      ) : null}
      {reservation.status === "REJECTED" ? (
        <Notice tone="red">
          {label(REJECT_LABELS, reservation.reject_reason)} {reservation.reject_detail}
        </Notice>
      ) : null}
      {reservation.overbooked ? (
        <Notice tone="red">
          Overbooking: a reserva veio de um canal externo e não havia quarto livre. Altere a estadia para atribuir um
          quarto ou entre em contato com o hóspede.
        </Notice>
      ) : null}
      {reservation.pending_change ? (
        <Notice tone="neutral">Alteração de estadia em processamento…</Notice>
      ) : null}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <Card>
            <CardHeader title="Estadia" />
            <dl className="grid grid-cols-2 gap-x-6 gap-y-4 px-5 py-4 text-sm sm:grid-cols-4">
              <Item term="Entrada" value={formatDate(reservation.check_in)} />
              <Item term="Saída" value={formatDate(reservation.check_out)} />
              <Item term="Noites" value={String(reservation.nights)} />
              <Item term="Quarto" value={reservation.room_name || "—"} />
              <Item term="Hóspedes" value={`${reservation.adults} adulto(s)${reservation.children ? `, ${reservation.children} criança(s)` : ""}`} />
              <Item term="Total" value={formatMoney(reservation.total_cents)} />
              <Item term="Sinal exigido" value={reservation.deposit_due_cents ? formatMoney(reservation.deposit_due_cents) : "Não exigido"} />
              <Item
                term="Prazo do sinal"
                value={reservation.hold_expires_at ? formatDateTime(reservation.hold_expires_at) : "—"}
              />
              <Item term="E-mail" value={reservation.holder.email || "—"} />
              <Item term="Telefone" value={reservation.holder.phone || "—"} />
              {reservation.notes ? <Item term="Observações" value={reservation.notes} wide /> : null}
              {reservation.cancel_reason ? <Item term="Motivo do cancelamento" value={reservation.cancel_reason} wide /> : null}
            </dl>
          </Card>

          {folio ? (
            <FolioCard folio={folio} reservation={reservation} />
          ) : waitingFolio ? (
            <Card className="px-5 py-4 text-sm text-ink-500">Abrindo a conta da reserva…</Card>
          ) : null}

          {["PENDING", "CONFIRMED", "CHECKED_IN", "CHECKED_OUT"].includes(reservation.status) ? (
            <Card>
              <CardHeader
                title="Ficha de registro (FNRH)"
                description="Exigida por lei para o check-in."
                action={reservation.registration_completed ? <Badge tone="brand">Preenchida</Badge> : <Badge tone="amber">Pendente</Badge>}
              />
              <div className="px-5 py-4">
                {registration.length > 0 ? (
                  <ul className="mb-4 divide-y divide-ink-100 rounded-lg border border-ink-200">
                    {registration.map(({ guest, card }) => (
                      <li key={card.id} className="flex flex-wrap items-center justify-between gap-2 px-4 py-2.5 text-sm">
                        <Link href={`/admin/guests/${guest.id}`} className="font-medium text-ink-900 hover:underline">
                          {guest.full_name}
                        </Link>
                        <span className="text-ink-500">
                          {label(DOCUMENT_LABELS, guest.document_type)} {guest.document_number}
                          {card.primary ? " · titular" : ""}
                        </span>
                      </li>
                    ))}
                  </ul>
                ) : null}
                {reservation.status !== "CHECKED_OUT" ? (
                  registration.length > 0 ? (
                    <details>
                      <summary className="cursor-pointer text-sm font-medium text-brand-700">Corrigir ou refazer a ficha</summary>
                      <div className="mt-4">
                        <RegistrationForm reservation={reservation} />
                      </div>
                    </details>
                  ) : (
                    <RegistrationForm reservation={reservation} />
                  )
                ) : null}
              </div>
            </Card>
          ) : null}
        </div>

        <div className="space-y-6">
          <Card>
            <CardHeader title="Ações" />
            <div className="px-5 py-4">
              <ReservationActions reservation={reservation} rooms={rooms.filter((room) => room.room_type_id === reservation.room_type_id)} />
            </div>
          </Card>

          <Card>
            <CardHeader title="Histórico" description="Os eventos gravados no Event Store, em ordem." />
            <ol className="space-y-3 px-5 py-4">
              {history.map((entry) => (
                <li key={entry.version} className="relative border-l-2 border-ink-200 pl-4">
                  <span className="absolute top-1.5 -left-[5px] size-2 rounded-full bg-brand-500" aria-hidden />
                  <p className="text-sm font-medium text-ink-900">{label(EVENT_LABELS, entry.type)}</p>
                  <p className="text-xs text-ink-500">
                    {formatDateTime(entry.occurred_at)} · {actorLabel(entry.actor)}
                  </p>
                </li>
              ))}
            </ol>
          </Card>
        </div>
      </div>
    </>
  );
}

function FolioCard({ folio, reservation }: { folio: Folio; reservation: Reservation }) {
  const paidCents = folio.paid_cents;
  const depositPendingCents = Math.max(folio.deposit_due_cents - paidCents, 0);

  return (
    <Card>
      <CardHeader
        title="Conta do hóspede"
        description={folio.status === "CLOSED" ? "Conta encerrada." : undefined}
        action={
          folio.balance_cents > 0 ? (
            <Badge tone="amber">Falta {formatMoney(folio.balance_cents)}</Badge>
          ) : folio.balance_cents < 0 ? (
            <Badge tone="blue">Crédito de {formatMoney(-folio.balance_cents)}</Badge>
          ) : (
            <Badge tone="brand">Quitada</Badge>
          )
        }
      />
      <Table>
        <thead>
          <tr>
            <Th>Data</Th>
            <Th>Lançamento</Th>
            <Th align="right">Valor</Th>
            <Th />
          </tr>
        </thead>
        <tbody>
          {folio.entries.map((entry) => {
            const isCredit = entry.kind === "PAYMENT" || entry.kind === "DISCOUNT";
            return (
              <tr key={entry.id} className={entry.voided ? "text-ink-400 line-through" : ""}>
                <Td>{formatDate(entry.business_date)}</Td>
                <Td>
                  {entry.kind === "PAYMENT" ? `Pagamento — ${label(METHOD_LABELS, entry.method)}` : entry.description}
                  {entry.fee_cents > 0 && !entry.voided ? (
                    <span className="ml-2 text-xs text-ink-500 no-underline">taxa {formatMoney(entry.fee_cents)}</span>
                  ) : null}
                  {entry.voided && entry.void_reason ? (
                    <span className="block text-xs no-underline">Anulado: {entry.void_reason}</span>
                  ) : null}
                </Td>
                <Td align="right" className={isCredit && !entry.voided ? "text-brand-700" : ""}>
                  {isCredit ? "− " : ""}
                  {formatMoney(entry.amount_cents)}
                </Td>
                <Td align="right">
                  {!entry.voided && folio.status === "OPEN" ? (
                    <VoidEntryForm folioId={folio.id} reservationId={reservation.id} entryId={entry.id} />
                  ) : null}
                </Td>
              </tr>
            );
          })}
        </tbody>
        <tfoot className="text-sm">
          <SummaryRow label="Total da conta" value={formatMoney(folio.total_due_cents)} />
          <SummaryRow label="Pago" value={formatMoney(paidCents)} />
          {folio.fees_cents > 0 ? <SummaryRow label="Taxas sobre os recebimentos" value={formatMoney(folio.fees_cents)} muted /> : null}
          <SummaryRow label="Saldo" value={formatMoney(folio.balance_cents)} strong />
        </tfoot>
      </Table>

      {folio.status === "OPEN" ? (
        <div className="grid gap-6 border-t border-ink-200 px-5 py-5">
          <div>
            <h3 className="mb-3 text-sm font-semibold text-ink-900">Receber pagamento</h3>
            <PaymentForm
              key={`${folio.entries.length}`}
              folioId={folio.id}
              reservationId={reservation.id}
              balanceCents={folio.balance_cents}
              depositPendingCents={reservation.status === "PENDING" ? depositPendingCents : 0}
              idempotencyKey={crypto.randomUUID()}
            />
          </div>
          <div>
            <h3 className="mb-3 text-sm font-semibold text-ink-900">Lançar consumo ou desconto</h3>
            <ChargeForm folioId={folio.id} reservationId={reservation.id} />
          </div>
        </div>
      ) : null}
    </Card>
  );
}

function SummaryRow({ label, value, strong, muted }: { label: string; value: string; strong?: boolean; muted?: boolean }) {
  return (
    <tr>
      <td colSpan={2} className={`px-5 py-1.5 text-right ${muted ? "text-ink-500" : "text-ink-700"}`}>
        {label}
      </td>
      <td className={`tabular px-5 py-1.5 text-right ${strong ? "font-semibold text-ink-900" : muted ? "text-ink-500" : ""}`}>{value}</td>
      <td />
    </tr>
  );
}

function Item({ term, value, wide }: { term: string; value: string; wide?: boolean }) {
  return (
    <div className={wide ? "col-span-2 sm:col-span-4" : ""}>
      <dt className="text-xs text-ink-500">{term}</dt>
      <dd className="mt-0.5 text-ink-900">{value}</dd>
    </div>
  );
}

function Notice({ tone, children }: { tone: "neutral" | "red"; children: React.ReactNode }) {
  return (
    <p
      role="status"
      className={`mb-6 rounded-lg px-4 py-3 text-sm ${tone === "red" ? "bg-red-50 text-red-800" : "bg-ink-100 text-ink-700"}`}
    >
      {children}
    </p>
  );
}

/** "user:<id>" → "equipe"; "system" → "sistema"; "channel:BOOKING_COM" → "Booking.com". */
function actorLabel(actor: string): string {
  if (actor.startsWith("user:")) return "equipe";
  if (actor.startsWith("channel:")) return label(SOURCE_LABELS, actor.slice("channel:".length));
  return "sistema";
}
