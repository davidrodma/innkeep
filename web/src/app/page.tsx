import Link from "next/link";
import { redirect } from "next/navigation";

import { SiteFooter, SiteHeader } from "@/components/public/site-chrome";
import { publishedProperties } from "@/features/booking/queries";

// A página inicial depende de dados que mudam (propriedades publicadas):
// renderização sob demanda, a cada requisição.
export const dynamic = "force-dynamic";

export default async function HomePage() {
  const properties = await publishedProperties().catch(() => []);
  // Com uma única propriedade publicada, a home é a página dela.
  if (properties.length === 1) redirect(`/p/${properties[0]!.slug}`);

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-5xl px-4 py-12">
        <h1 className="text-3xl font-semibold tracking-tight text-ink-900">Encontre sua hospedagem</h1>
        <p className="mt-2 text-ink-600">Reserve direto com a propriedade, sem intermediários.</p>

        {properties.length === 0 ? (
          <p className="mt-10 rounded-xl border border-ink-200 bg-white px-6 py-10 text-center text-sm text-ink-500">
            Nenhuma propriedade publicada ainda.
          </p>
        ) : (
          <ul className="mt-8 grid gap-4 sm:grid-cols-2">
            {properties.map((property) => (
              <li key={property.id}>
                <Link
                  href={`/p/${property.slug}`}
                  className="block rounded-xl border border-ink-200 bg-white p-5 transition-colors hover:border-brand-300"
                >
                  <p className="text-lg font-semibold text-ink-900">{property.name}</p>
                  <p className="mt-1 text-sm text-ink-500">
                    {[property.address.city, property.address.state].filter(Boolean).join(" — ")}
                  </p>
                  {property.description ? <p className="mt-3 line-clamp-2 text-sm text-ink-600">{property.description}</p> : null}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </main>
      <SiteFooter />
    </>
  );
}
