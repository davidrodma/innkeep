import type { Metadata } from "next";
import Link from "next/link";

import { buttonClass } from "@/components/ui/button";
import { Input } from "@/components/ui/form";
import { Card, EmptyState, PageHeader, Table, Td, Th } from "@/components/ui/layout";
import { api, query } from "@/lib/api";
import { DOCUMENT_LABELS, label } from "@/lib/labels";
import type { Guest, List } from "@/lib/types";

export const metadata: Metadata = { title: "Hóspedes" };

type Props = { searchParams: Promise<{ q?: string }> };

export default async function GuestsPage({ searchParams }: Props) {
  const { q = "" } = await searchParams;
  // Busca por documento exato quando o texto parece um CPF (11 dígitos).
  const digits = q.replace(/\D/g, "");
  const path =
    digits.length === 11 && /^[\d.\-\s]+$/.test(q)
      ? `/v1/guests${query({ document_type: "CPF", document_number: digits })}`
      : `/v1/guests${query({ q, limit: 100 })}`;
  const guests = await api<List<Guest>>(path);

  return (
    <>
      <PageHeader title="Hóspedes" description="Cadastro único de hóspedes, reaproveitado a cada nova estadia." />
      <form method="get" className="mb-4 flex flex-wrap items-end gap-3">
        <div className="w-full sm:w-96">
          <label htmlFor="q" className="mb-1 block text-sm font-medium text-ink-700">
            Buscar
          </label>
          <Input name="q" defaultValue={q} placeholder="Nome, e-mail, telefone ou CPF" />
        </div>
        <button type="submit" className={buttonClass("secondary")}>
          Buscar
        </button>
      </form>

      <Card>
        {guests.items.length === 0 ? (
          <EmptyState
            title="Nenhum hóspede encontrado"
            description="Os hóspedes são cadastrados ao preencher a ficha de registro (FNRH) de uma reserva."
          />
        ) : (
          <Table>
            <thead>
              <tr>
                <Th>Nome</Th>
                <Th>Documento</Th>
                <Th>Contato</Th>
                <Th>Cidade</Th>
              </tr>
            </thead>
            <tbody>
              {guests.items.map((guest) => (
                <tr key={guest.id} className="hover:bg-ink-50">
                  <Td>
                    <Link href={`/admin/guests/${guest.id}`} className="font-medium text-ink-900 hover:underline">
                      {guest.full_name}
                    </Link>
                  </Td>
                  <Td className="font-mono text-xs">
                    {label(DOCUMENT_LABELS, guest.document_type)} {guest.document_number}
                  </Td>
                  <Td>{guest.email || guest.phone || "—"}</Td>
                  <Td>{[guest.address.city, guest.address.state].filter(Boolean).join("/") || "—"}</Td>
                </tr>
              ))}
            </tbody>
          </Table>
        )}
      </Card>
      <p className="mt-3 text-xs text-ink-500">
        Nas listagens o documento aparece mascarado. O número completo só é exibido no cadastro, e cada consulta fica
        registrada (LGPD).
      </p>
    </>
  );
}
