import type { Metadata } from "next";

import { SiteFooter, SiteHeader } from "@/components/public/site-chrome";
import { LookupForm } from "@/features/booking/lookup-form";

export const metadata: Metadata = { title: "Minha reserva" };

export default function LookupPage() {
  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-sm px-4 py-12">
        <h1 className="text-2xl font-semibold tracking-tight text-ink-900">Minha reserva</h1>
        <p className="mt-1 text-sm text-ink-600">Acompanhe a situação da sua reserva e as instruções de pagamento.</p>
        <div className="mt-6 rounded-xl border border-ink-200 bg-white p-6">
          <LookupForm />
        </div>
      </main>
      <SiteFooter />
    </>
  );
}
