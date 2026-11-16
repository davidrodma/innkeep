"use client";

import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";

const TABS = [
  { href: "/admin/finance", label: "Resumo" },
  { href: "/admin/finance/expenses", label: "Despesas" },
  { href: "/admin/finance/cash", label: "Caixa" },
  { href: "/admin/finance/receivables", label: "A receber" },
  { href: "/admin/finance/fees", label: "Taxas" },
];

export function FinanceTabs() {
  const pathname = usePathname();
  const month = useSearchParams().get("month");
  return (
    <nav aria-label="Seções do financeiro" className="mb-6 flex gap-1 overflow-x-auto border-b border-ink-200">
      {TABS.map((tab) => {
        const active = pathname === tab.href;
        return (
          <Link
            key={tab.href}
            href={month ? `${tab.href}?month=${month}` : tab.href}
            aria-current={active ? "page" : undefined}
            className={`-mb-px shrink-0 border-b-2 px-3 py-2 text-sm font-medium ${
              active ? "border-brand-600 text-brand-800" : "border-transparent text-ink-500 hover:text-ink-800"
            }`}
          >
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}
