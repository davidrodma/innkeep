import type { ReactNode } from "react";

import { STATUS_LABELS } from "@/lib/labels";
import type { ReservationStatus } from "@/lib/types";

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <section className={`rounded-xl border border-ink-200 bg-white ${className}`}>{children}</section>;
}

export function CardHeader({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-3 border-b border-ink-200 px-5 py-4">
      <div>
        <h2 className="text-base font-semibold text-ink-900">{title}</h2>
        {description ? <p className="mt-0.5 text-sm text-ink-500">{description}</p> : null}
      </div>
      {action}
    </header>
  );
}

export function PageHeader({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-ink-900">{title}</h1>
        {description ? <p className="mt-1 text-sm text-ink-500">{description}</p> : null}
      </div>
      {action ? <div className="flex flex-wrap items-center gap-2">{action}</div> : null}
    </div>
  );
}

export function EmptyState({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="px-6 py-12 text-center">
      <p className="text-sm font-medium text-ink-800">{title}</p>
      {description ? <p className="mx-auto mt-1 max-w-md text-sm text-ink-500">{description}</p> : null}
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  );
}

type Tone = "neutral" | "brand" | "amber" | "red" | "blue" | "violet";

const TONES: Record<Tone, string> = {
  neutral: "bg-ink-100 text-ink-700",
  brand: "bg-brand-100 text-brand-800",
  amber: "bg-amber-100 text-amber-800",
  red: "bg-red-100 text-red-700",
  blue: "bg-sky-100 text-sky-800",
  violet: "bg-violet-100 text-violet-800",
};

export function Badge({ children, tone = "neutral" }: { children: ReactNode; tone?: Tone }) {
  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${TONES[tone]}`}>
      {children}
    </span>
  );
}

export const STATUS_TONES: Record<ReservationStatus, Tone> = {
  REQUESTED: "neutral",
  PENDING: "amber",
  CONFIRMED: "brand",
  CHECKED_IN: "blue",
  CHECKED_OUT: "neutral",
  CANCELLED: "red",
  REJECTED: "red",
  EXPIRED: "red",
  NO_SHOW: "violet",
};

export function StatusBadge({ status }: { status: ReservationStatus }) {
  return <Badge tone={STATUS_TONES[status]}>{STATUS_LABELS[status]}</Badge>;
}

/** Indicador numérico do painel ("Chegadas hoje: 3"). */
export function Stat({ label, value, hint, tone = "neutral" }: { label: string; value: string; hint?: string; tone?: "neutral" | "good" | "bad" }) {
  const color = tone === "good" ? "text-brand-700" : tone === "bad" ? "text-red-600" : "text-ink-900";
  return (
    <div className="rounded-xl border border-ink-200 bg-white px-5 py-4">
      <p className="text-sm text-ink-500">{label}</p>
      <p className={`tabular mt-1 text-2xl font-semibold tracking-tight ${color}`}>{value}</p>
      {hint ? <p className="mt-1 text-xs text-ink-500">{hint}</p> : null}
    </div>
  );
}

/** Tabela com rolagem horizontal própria em telas estreitas. */
export function Table({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[640px] text-left text-sm">{children}</table>
    </div>
  );
}

export function Th({ children, align = "left" }: { children?: ReactNode; align?: "left" | "right" }) {
  return (
    <th className={`border-b border-ink-200 px-5 py-2.5 text-xs font-medium text-ink-500 ${align === "right" ? "text-right" : ""}`}>
      {children}
    </th>
  );
}

export function Td({ children, align = "left", className = "" }: { children?: ReactNode; align?: "left" | "right"; className?: string }) {
  return (
    <td className={`border-b border-ink-100 px-5 py-3 align-middle ${align === "right" ? "tabular text-right" : ""} ${className}`}>
      {children}
    </td>
  );
}
