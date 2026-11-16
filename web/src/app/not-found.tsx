import Link from "next/link";

import { buttonClass } from "@/components/ui/button";

export default function NotFound() {
  return (
    <main className="grid min-h-dvh place-items-center px-4">
      <div className="text-center">
        <p className="text-sm font-semibold text-brand-700">404</p>
        <h1 className="mt-1 text-2xl font-semibold text-ink-900">Página não encontrada</h1>
        <p className="mt-2 text-sm text-ink-600">O endereço pode ter mudado ou não existir mais.</p>
        <Link href="/" className={`${buttonClass("primary")} mt-6`}>
          Ir para o início
        </Link>
      </div>
    </main>
  );
}
