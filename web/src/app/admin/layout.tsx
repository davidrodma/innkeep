import type { Metadata } from "next";

import { Sidebar } from "@/components/admin/sidebar";
import { availableProperties, currentUser } from "@/lib/session";
import { PROPERTY_COOKIE } from "@/lib/session-cookies";
import { cookies } from "next/headers";

export const metadata: Metadata = { title: { default: "Painel", template: "%s · InnKeep" } };

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  // Promise.all: as duas consultas saem em paralelo, não uma depois da outra.
  const [user, properties] = await Promise.all([currentUser(), availableProperties()]);
  const selected = (await cookies()).get(PROPERTY_COOKIE)?.value;
  const current = properties.find((property) => property.id === selected) ?? properties[0];

  return (
    <div className="md:flex">
      <Sidebar
        userName={user.name}
        permissions={user.permissions}
        properties={properties.map((property) => ({ id: property.id, name: property.name }))}
        currentPropertyId={current?.id ?? ""}
      />
      <main className="min-w-0 flex-1 px-4 py-6 md:px-8 md:py-8">
        <div className="mx-auto max-w-6xl">{children}</div>
      </main>
    </div>
  );
}
