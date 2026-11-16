import { Suspense } from "react";

import { FinanceTabs } from "@/components/admin/finance-tabs";
import { PageHeader } from "@/components/ui/layout";

export default function FinanceLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <PageHeader title="Financeiro" description="Receitas, despesas, caixa em espécie e contas a receber." />
      {/* As abas leem a URL no navegador (useSearchParams): o Suspense delimita
          essa parte, para o restante da página renderizar sem esperar por ela. */}
      <Suspense fallback={<div className="mb-6 h-10 border-b border-ink-200" />}>
        <FinanceTabs />
      </Suspense>
      {children}
    </>
  );
}
