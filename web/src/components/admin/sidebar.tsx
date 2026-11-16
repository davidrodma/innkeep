"use client";

import {
  BedDouble,
  Building2,
  Cable,
  CalendarDays,
  ClipboardList,
  LayoutDashboard,
  LogOut,
  Sparkles,
  Tags,
  UserCog,
  Users,
  Wallet,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { logout, selectProperty } from "@/features/auth/actions";

type Item = { href: string; label: string; icon: LucideIcon; permission?: string };

// Cada item aparece só para quem tem a permissão — a mesma permissão que a
// API exige. Esconder o link é conforto; quem protege de verdade é a API.
const ITEMS: Item[] = [
  { href: "/admin", label: "Painel", icon: LayoutDashboard },
  { href: "/admin/calendar", label: "Calendário", icon: CalendarDays, permission: "reservation:read" },
  { href: "/admin/reservations", label: "Reservas", icon: ClipboardList, permission: "reservation:read" },
  { href: "/admin/guests", label: "Hóspedes", icon: Users, permission: "guest:read" },
  { href: "/admin/rates", label: "Tarifas", icon: Tags, permission: "inventory:read" },
  { href: "/admin/finance", label: "Financeiro", icon: Wallet, permission: "finance:read" },
  { href: "/admin/channels", label: "Canais", icon: Cable, permission: "channel:read" },
  { href: "/admin/housekeeping", label: "Governança", icon: Sparkles, permission: "housekeeping:write" },
  { href: "/admin/property", label: "Propriedade", icon: Building2, permission: "property:write" },
  { href: "/admin/users", label: "Equipe e conta", icon: UserCog },
];

type Props = {
  userName: string;
  permissions: string[];
  properties: { id: string; name: string }[];
  currentPropertyId: string;
};

export function Sidebar({ userName, permissions, properties, currentPropertyId }: Props) {
  const pathname = usePathname();
  const items = ITEMS.filter((item) => !item.permission || permissions.includes(item.permission));

  return (
    <aside className="flex shrink-0 flex-col border-ink-200 bg-white md:h-dvh md:w-60 md:border-r md:sticky md:top-0">
      <div className="flex items-center gap-2 px-5 pt-5 pb-3">
        <BedDouble className="size-5 text-brand-600" aria-hidden />
        <span className="text-lg font-semibold tracking-tight text-ink-900">InnKeep</span>
      </div>

      {properties.length > 1 ? (
        <form action={selectProperty} className="px-3 pb-2">
          <label htmlFor="property_id" className="sr-only">
            Propriedade
          </label>
          <select
            id="property_id"
            name="property_id"
            defaultValue={currentPropertyId}
            onChange={(event) => event.currentTarget.form?.requestSubmit()}
            className="h-9 w-full rounded-lg border border-ink-300 bg-white px-2 text-sm"
          >
            {properties.map((property) => (
              <option key={property.id} value={property.id}>
                {property.name}
              </option>
            ))}
          </select>
        </form>
      ) : (
        <p className="truncate px-5 pb-2 text-xs text-ink-500">{properties[0]?.name}</p>
      )}

      <nav aria-label="Menu principal" className="flex gap-1 overflow-x-auto px-3 pb-2 md:flex-1 md:flex-col md:overflow-visible">
        {items.map((item) => {
          const active = item.href === "/admin" ? pathname === "/admin" : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={active ? "page" : undefined}
              className={`flex shrink-0 items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition-colors ${
                active ? "bg-brand-50 text-brand-800" : "text-ink-600 hover:bg-ink-100 hover:text-ink-900"
              }`}
            >
              <item.icon className="size-4" aria-hidden />
              {item.label}
            </Link>
          );
        })}
      </nav>

      <div className="hidden border-t border-ink-200 p-3 md:block">
        <p className="truncate px-2.5 pb-1 text-xs text-ink-500">{userName}</p>
        <form action={logout}>
          <button
            type="submit"
            className="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium text-ink-600 hover:bg-ink-100 hover:text-ink-900"
          >
            <LogOut className="size-4" aria-hidden />
            Sair
          </button>
        </form>
      </div>
    </aside>
  );
}
