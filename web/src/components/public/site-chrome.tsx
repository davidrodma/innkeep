import { BedDouble } from "lucide-react";
import Link from "next/link";

/** Cabeçalho e rodapé do site público de reservas. */
export function SiteHeader({ title = "InnKeep", href = "/" }: { title?: string; href?: string }) {
  return (
    <header className="border-b border-ink-200 bg-white">
      <div className="mx-auto flex h-14 max-w-5xl items-center justify-between gap-4 px-4">
        <Link href={href} className="flex items-center gap-2 font-semibold tracking-tight text-ink-900">
          <BedDouble className="size-5 text-brand-600" aria-hidden />
          {title}
        </Link>
        <nav className="flex items-center gap-4 text-sm">
          <Link href="/booking" className="text-ink-600 hover:text-ink-900">
            Minha reserva
          </Link>
        </nav>
      </div>
    </header>
  );
}

export function SiteFooter() {
  return (
    <footer className="mt-16 border-t border-ink-200 bg-white">
      <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-2 px-4 py-6 text-xs text-ink-500">
        <p>Reservas pelo InnKeep — software livre de gestão hoteleira.</p>
        <Link href="/login" className="hover:text-ink-800">
          Acesso da equipe
        </Link>
      </div>
    </footer>
  );
}
