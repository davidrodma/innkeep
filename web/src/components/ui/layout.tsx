import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { STATUS_LABELS } from "@/lib/labels";
import type { ReservationStatus } from "@/lib/types";

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <section className={`rounded-md bg-white shadow-[0_1px_3px_rgba(15,23,42,0.08),0_1px_2px_-1px_rgba(15,23,42,0.06)] ${className}`}>{children}</section>;
}

export function CardHeader({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-start justify-between gap-3 border-b border-ink-100 px-5 py-4">
      <div>
        <h2 className="text-lg font-bold text-ink-900">{title}</h2>
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
        <h1 className="text-2xl font-extrabold tracking-tight text-ink-900">{title}</h1>
        {description ? <p className="mt-1 text-sm text-ink-500">{description}</p> : null}
      </div>
      {action ? <div className="flex flex-wrap items-center gap-2">{action}</div> : null}
    </div>
  );
}

export function EmptyState({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return (
    <div className="px-6 py-12 text-center">
      <p className="text-sm font-semibold text-ink-800">{title}</p>
      {description ? <p className="mx-auto mt-1 max-w-md text-sm text-ink-500">{description}</p> : null}
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  );
}

type Tone = "neutral" | "brand" | "amber" | "red" | "blue" | "violet";

const TONES: Record<Tone, string> = {
  neutral: "bg-ink-100 text-ink-700",
  brand: "bg-success-soft text-success-ink",
  amber: "bg-warning-soft text-warning-ink",
  red: "bg-danger-soft text-danger-ink",
  blue: "bg-info-soft text-info-ink",
  violet: "bg-accent-soft text-accent-ink",
};

export function Badge({ children, tone = "neutral" }: { children: ReactNode; tone?: Tone }) {
  return (
    <span className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-bold ${TONES[tone]}`}>
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

type StatTone = "neutral" | "good" | "bad";

const STAT_ICON_TONES: Record<StatTone, string> = {
  neutral: "bg-brand-50 text-brand-600",
  good: "bg-success-soft text-success-ink",
  bad: "bg-danger-soft text-danger-ink",
};

/**
 * Indicador numérico do painel ("Chegadas hoje: 3"). O ícone e a barra de
 * progresso são opcionais; `progress` vai de 0 a 100 (ex.: ocupação).
 */
export function Stat({
  label,
  value,
  hint,
  tone = "neutral",
  icon: Icon,
  progress,
}: {
  label: string;
  value: string;
  hint?: string;
  tone?: StatTone;
  icon?: LucideIcon;
  progress?: number;
}) {
  const color = tone === "good" ? "text-success-ink" : tone === "bad" ? "text-danger-ink" : "text-ink-900";
  return (
    <div className="rounded-md bg-white p-4 sm:p-5 shadow-[0_1px_3px_rgba(15,23,42,0.08),0_1px_2px_-1px_rgba(15,23,42,0.06)]">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-ink-500">{label}</p>
          <p className={`tabular mt-1 text-3xl font-extrabold tracking-tight ${color}`}>{value}</p>
        </div>
        {Icon ? (
          <span className={`hidden size-11 shrink-0 place-items-center rounded-full sm:grid ${STAT_ICON_TONES[tone]}`}>
            <Icon className="size-5" aria-hidden />
          </span>
        ) : null}
      </div>
      {progress !== undefined ? (
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-ink-100" aria-hidden>
          <div className="h-full rounded-full bg-brand-600" style={{ width: `${Math.min(100, Math.max(0, progress))}%` }} />
        </div>
      ) : null}
      {hint ? <p className="mt-2 text-xs font-semibold text-ink-500">{hint}</p> : null}
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
    <th className={`bg-ink-50 px-5 py-3 text-xs font-bold tracking-wide text-ink-600 uppercase ${align === "right" ? "text-right" : ""}`}>
      {children}
    </th>
  );
}

export function Td({ children, align = "left", className = "" }: { children?: ReactNode; align?: "left" | "right"; className?: string }) {
  return (
    <td className={`border-b border-ink-100 px-5 py-3.5 align-middle ${align === "right" ? "tabular text-right" : ""} ${className}`}>
      {children}
    </td>
  );
}
