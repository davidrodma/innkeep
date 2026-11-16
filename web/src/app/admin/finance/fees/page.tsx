import type { Metadata } from "next";

import { Card, CardHeader } from "@/components/ui/layout";
import { FeeRulesForm } from "@/features/finance/forms";
import { api, query } from "@/lib/api";
import { currentProperty } from "@/lib/session";
import type { FeeRule, List } from "@/lib/types";

export const metadata: Metadata = { title: "Taxas" };

export default async function FeesPage() {
  const property = await currentProperty();
  const rules = (await api<List<FeeRule>>(`/v1/fee-rules${query({ property_id: property.id })}`)).items;

  return (
    <Card>
      <CardHeader
        title="Taxas por forma de pagamento"
        description="Percentual descontado de cada recebimento: taxa da maquininha, do link de pagamento ou comissão do canal. Usado para calcular o valor líquido."
      />
      <div className="px-5 py-4">
        <FeeRulesForm propertyId={property.id} rules={rules} />
      </div>
    </Card>
  );
}
