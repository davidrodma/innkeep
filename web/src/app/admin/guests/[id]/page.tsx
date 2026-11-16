import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { Badge, Card, CardHeader, EmptyState, PageHeader, Table, Td, Th } from "@/components/ui/layout";
import { anonymizeGuest } from "@/features/guests/actions";
import { ApiError, api } from "@/lib/api";
import { formatDate, formatDateTime } from "@/lib/format";
import { DOCUMENT_LABELS, TRANSPORT_LABELS, TRIP_PURPOSE_LABELS, label } from "@/lib/labels";
import type { Guest, List, RegistrationCard } from "@/lib/types";

export const metadata: Metadata = { title: "Hóspede" };

type Props = { params: Promise<{ id: string }> };

export default async function GuestPage({ params }: Props) {
  const { id } = await params;
  let guest: Guest;
  try {
    guest = await api<Guest>(`/v1/guests/${id}`);
  } catch (error) {
    if (error instanceof ApiError && (error.status === 404 || error.status === 400)) notFound();
    throw error;
  }
  const stays = (await api<List<RegistrationCard>>(`/v1/guests/${id}/stays`)).items;

  const fields: [string, string][] = [
    ["Documento", `${label(DOCUMENT_LABELS, guest.document_type)} ${guest.document_number}`],
    ["Nascimento", guest.birth_date ? formatDate(guest.birth_date) : "—"],
    ["Nacionalidade", guest.nationality || "—"],
    ["Profissão", guest.profession || "—"],
    ["E-mail", guest.email || "—"],
    ["Telefone", guest.phone || "—"],
    [
      "Endereço",
      [guest.address.line, guest.address.district, guest.address.city, guest.address.state, guest.address.postal_code].filter(Boolean).join(", ") || "—",
    ],
  ];

  return (
    <>
      <PageHeader title={guest.full_name} description={guest.anonymized ? "Dados pessoais eliminados a pedido do titular." : "Cadastro do hóspede"} />
      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader title="Dados pessoais" />
          <dl className="grid gap-x-6 gap-y-4 px-5 py-4 text-sm sm:grid-cols-2">
            {fields.map(([term, value]) => (
              <div key={term}>
                <dt className="text-xs text-ink-500">{term}</dt>
                <dd className="mt-0.5 text-ink-900">{value}</dd>
              </div>
            ))}
          </dl>
        </Card>

        <Card>
          <CardHeader title="Privacidade (LGPD)" />
          <div className="space-y-3 px-5 py-4 text-sm text-ink-600">
            <p>
              O titular pode pedir a eliminação dos dados pessoais. As fichas de hospedagem continuam existindo, sem os dados
              que identificam a pessoa.
            </p>
            <p className="text-xs">
              Atenção: a legislação de turismo exige guardar a FNRH por um período. Verifique o prazo antes de anonimizar.
            </p>
            {!guest.anonymized ? (
              <details>
                <summary className="cursor-pointer font-medium text-red-700">Anonimizar hóspede</summary>
                <form action={anonymizeGuest} className="mt-3">
                  <input type="hidden" name="id" value={guest.id} />
                  <button type="submit" className="rounded-lg bg-red-600 px-3 py-2 text-sm font-medium text-white hover:bg-red-700">
                    Confirmar anonimização
                  </button>
                </form>
              </details>
            ) : (
              <Badge>Anonimizado</Badge>
            )}
          </div>
        </Card>
      </div>

      <Card className="mt-6">
        <CardHeader title="Estadias" />
        {stays.length === 0 ? (
          <EmptyState title="Nenhuma estadia registrada" />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Estadia</Th>
                <Th>Motivo</Th>
                <Th>Transporte</Th>
                <Th>Procedência</Th>
                <Th>Entrada efetiva</Th>
              </tr>
            </thead>
            <tbody>
              {stays.map((stay) => (
                <tr key={stay.id}>
                  <Td>
                    <Link href={`/admin/reservations/${stay.reservation_id}`} className="text-brand-700 hover:underline">
                      {formatDate(stay.check_in)} a {formatDate(stay.check_out)}
                    </Link>
                  </Td>
                  <Td>{label(TRIP_PURPOSE_LABELS, stay.purpose_of_trip)}</Td>
                  <Td>{label(TRANSPORT_LABELS, stay.transport)}</Td>
                  <Td>{[stay.origin.city, stay.origin.state].filter(Boolean).join("/") || "—"}</Td>
                  <Td>{stay.checked_in_at ? formatDateTime(stay.checked_in_at) : "—"}</Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>
    </>
  );
}
