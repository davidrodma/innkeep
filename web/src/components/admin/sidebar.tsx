"use client";

import {
  BedDouble,
  Building2,
  Cable,
  CalendarDays,
  ClipboardList,
  LayoutDashboard,
  Sparkles,
  Tags,
  UserCog,
  Users,
  Wallet,
  X,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { selectProperty } from "@/features/auth/actions";

type Item = { href: string; label: string; icon: LucideIcon; permission?: string };
type Section = { title?: string; items: Item[] };

// Cada item aparece só para quem tem a permissão — a mesma permissão que a
// API exige. Esconder o link é conforto; quem protege de verdade é a API.
// As seções agrupam o menu pelo momento de uso: o dia a dia da recepção, a
// parte comercial e a administração.
const SECTIONS: Section[] = [
  {
    items: [
      { href: "/admin", label: "Painel", icon: LayoutDashboard },
      { href: "/admin/calendar", label: "Calendário", icon: CalendarDays, permission: "reservation:read" },
    ],
  },
  {
    title: "Operação",
    items: [
      { href: "/admin/reservations", label: "Reservas", icon: ClipboardList, permission: "reservation:read" },
      { href: "/admin/guests", label: "Hóspedes", icon: Users, permission: "guest:read" },
      { href: "/admin/housekeeping", label: "Governança", icon: Sparkles, permission: "housekeeping:write" },
    ],
  },
  {
    title: "Comercial",
    items: [
      { href: "/admin/rates", label: "Tarifas", icon: Tags, permission: "inventory:read" },
      { href: "/admin/channels", label: "Canais", icon: Cable, permission: "channel:read" },
    ],
  },
  {
    title: "Gestão",
    items: [
      { href: "/admin/finance", label: "Financeiro", icon: Wallet, permission: "finance:read" },
      { href: "/admin/property", label: "Propriedade", icon: Building2, permission: "property:write" },
      { href: "/admin/users", label: "Equipe e conta", icon: UserCog },
    ],
  },
];

type Props = {
  permissions: string[];
  properties: { id: string; name: string }[];
  currentPropertyId: string;
  /** No celular o menu é uma gaveta: `open` a mostra e `onClose` a fecha. */
  open: boolean;
  onClose: () => void;
};

export function Sidebar({ permissions, properties, currentPropertyId, open, onClose }: Props) {
  const pathname = usePathname();
  const sections = SECTIONS.map((section) => ({
    ...section,
    items: section.items.filter((item) => !item.permission || permissions.includes(item.permission)),
  })).filter((section) => section.items.length > 0);

  return (
    <aside
      id="admin-sidebar"
      className={`fixed inset-y-0 left-0 z-50 flex w-[260px] flex-col bg-white shadow-[5px_0_25px_0_rgba(94,92,154,0.1)] transition-transform duration-300 lg:translate-x-0 ${
        open ? "translate-x-0" : "-translate-x-full"
      }`}
    >
      <div className="flex h-16 shrink-0 items-center justify-between px-4">
        <Link href="/admin" onClick={onClose} className="flex items-center gap-2.5">
          <span className="grid size-9 place-items-center rounded-lg bg-brand-600 text-white shadow-[0_8px_16px_-8px] shadow-brand-600">
            <BedDouble className="size-5" aria-hidden />
          </span>
          <span className="text-xl font-extrabold tracking-tight text-ink-900">InnKeep</span>
        </Link>
        <button
          type="button"
          onClick={onClose}
          className="grid size-8 place-items-center rounded-full text-ink-500 hover:bg-ink-100 hover:text-ink-900 lg:hidden"
          aria-label="Fechar menu"
        >
          <X className="size-5" aria-hidden />
        </button>
      </div>

      <div className="px-4 pb-3">
        {properties.length > 1 ? (
          <form action={selectProperty}>
            <label htmlFor="property_id" className="mb-1 block text-xs font-bold tracking-wide text-ink-500 uppercase">
              Propriedade
            </label>
            <select
              id="property_id"
              name="property_id"
              defaultValue={currentPropertyId}
              onChange={(event) => event.currentTarget.form?.requestSubmit()}
              className="h-9 w-full rounded-md border border-ink-200 bg-white px-2 text-sm font-semibold text-ink-800 focus:border-brand-500"
            >
              {properties.map((property) => (
                <option key={property.id} value={property.id}>
                  {property.name}
                </option>
              ))}
            </select>
          </form>
        ) : (
          <p className="flex items-center gap-2 truncate rounded-md bg-ink-50 px-3 py-2 text-sm font-semibold text-ink-700">
            <Building2 className="size-4 shrink-0 text-ink-400" aria-hidden />
            <span className="truncate">{properties[0]?.name}</span>
          </p>
        )}
      </div>

      <nav aria-label="Menu principal" className="flex-1 overflow-y-auto pb-6">
        {sections.map((section, index) => (
          <div key={section.title ?? index}>
            {section.title ? (
              <h2 className="mt-2 mb-1 bg-ink-50 px-6 py-2.5 text-xs font-extrabold tracking-wider text-ink-500 uppercase">
                {section.title}
              </h2>
            ) : null}
            <ul className="space-y-0.5 px-4">
              {section.items.map((item) => {
                const active = item.href === "/admin" ? pathname === "/admin" : pathname.startsWith(item.href);
                return (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      onClick={onClose}
                      aria-current={active ? "page" : undefined}
                      className={`group flex items-center gap-3 rounded-md px-3 py-2.5 text-[15px] font-semibold transition-colors ${
                        active ? "bg-brand-50 text-brand-700" : "text-ink-600 hover:bg-ink-100 hover:text-ink-900"
                      }`}
                    >
                      <item.icon
                        className={`size-5 shrink-0 ${active ? "text-brand-600" : "text-ink-400 group-hover:text-ink-700"}`}
                        aria-hidden
                      />
                      {item.label}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>
    </aside>
  );
}
