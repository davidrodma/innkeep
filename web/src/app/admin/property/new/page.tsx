import type { Metadata } from "next";

import { Card, PageHeader } from "@/components/ui/layout";
import { PropertyForm } from "@/features/property/property-form";

export const metadata: Metadata = { title: "Nova propriedade" };

export default function NewPropertyPage() {
  return (
    <>
      <PageHeader title="Nova propriedade" description="Depois do cadastro, adicione os tipos de acomodação e os quartos." />
      <Card className="p-5 sm:p-6">
        <PropertyForm />
      </Card>
    </>
  );
}
