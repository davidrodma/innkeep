import Link from "next/link";

import { buttonClass } from "@/components/ui/button";

/** Navegação entre meses (?month=AAAA-MM), usada nas telas do financeiro. */
export function MonthNav({ path, label, previousMonth, nextMonth }: { path: string; label: string; previousMonth: string; nextMonth: string }) {
  return (
    <div className="mb-4 flex items-center justify-between gap-2">
      <Link href={`${path}?month=${previousMonth}`} className={buttonClass("secondary", "sm")} aria-label="Mês anterior">
        ←
      </Link>
      <p className="text-sm font-semibold text-ink-800 first-letter:uppercase">{label}</p>
      <Link href={`${path}?month=${nextMonth}`} className={buttonClass("secondary", "sm")} aria-label="Próximo mês">
        →
      </Link>
    </div>
  );
}
