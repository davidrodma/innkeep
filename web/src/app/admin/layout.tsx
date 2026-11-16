import type { Metadata } from "next";
import { cookies } from "next/headers";

import { AdminShell } from "@/components/admin/admin-shell";
import { nunito } from "@/lib/fonts";
import { ROLE_LABELS } from "@/lib/labels";
import { availableProperties, currentUser } from "@/lib/session";
import { PROPERTY_COOKIE } from "@/lib/session-cookies";

export const metadata: Metadata = { title: { default: "Painel", template: "%s · InnKeep" } };

export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  // Promise.all: as duas consultas saem em paralelo, não uma depois da outra.
  const [user, properties] = await Promise.all([currentUser(), availableProperties()]);
  const selected = (await cookies()).get(PROPERTY_COOKIE)?.value;
  const current = properties.find((property) => property.id === selected) ?? properties[0];

  return (
    // A classe da fonte define a variável --font-nunito, lida pelo tema do painel.
    <div className={nunito.variable}>
      <AdminShell
        userName={user.name}
        roleLabel={ROLE_LABELS[user.role] ?? user.role}
        permissions={user.permissions}
        properties={properties.map((property) => ({ id: property.id, name: property.name }))}
        currentPropertyId={current?.id ?? ""}
      >
        {children}
      </AdminShell>
    </div>
  );
}
