"use client";

import { ChevronDown, ExternalLink, LogOut, Menu, UserCog } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { logout } from "@/features/auth/actions";

import { Sidebar } from "./sidebar";

type Props = {
  userName: string;
  roleLabel: string;
  permissions: string[];
  properties: { id: string; name: string }[];
  currentPropertyId: string;
  children: ReactNode;
};

/**
 * Esqueleto do painel: menu lateral fixo, barra superior e área de conteúdo.
 *
 * É um Client Component só porque guarda dois estados de interface (gaveta do
 * menu no celular e menu do usuário). Os dados chegam prontos do layout, que
 * roda no servidor; as páginas continuam sendo Server Components (`children`).
 */
export function AdminShell({ userName, roleLabel, permissions, properties, currentPropertyId, children }: Props) {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <div className="admin-theme min-h-dvh">
      <Sidebar
        permissions={permissions}
        properties={properties}
        currentPropertyId={currentPropertyId}
        open={menuOpen}
        onClose={() => setMenuOpen(false)}
      />
      {/* Fundo escurecido atrás da gaveta, só no celular; clicar nele fecha o menu. */}
      {menuOpen ? (
        <div className="fixed inset-0 z-40 bg-ink-900/60 lg:hidden" onClick={() => setMenuOpen(false)} aria-hidden />
      ) : null}

      <div className="flex min-h-dvh flex-col lg:ml-[260px]">
        <header className="sticky top-0 z-30 flex h-16 items-center gap-3 bg-white px-4 shadow-sm md:px-6">
          <button
            type="button"
            onClick={() => setMenuOpen(true)}
            className="grid size-9 place-items-center rounded-full bg-ink-100 text-ink-700 hover:bg-brand-50 hover:text-brand-700 lg:hidden"
            aria-label="Abrir menu"
            aria-controls="admin-sidebar"
            aria-expanded={menuOpen}
          >
            <Menu className="size-5" aria-hidden />
          </button>

          <div className="ml-auto flex items-center gap-2">
            <Link
              href="/"
              target="_blank"
              className="hidden items-center gap-1.5 rounded-full bg-ink-100 px-3.5 py-2 text-sm font-semibold text-ink-700 hover:bg-brand-50 hover:text-brand-700 sm:flex"
            >
              <ExternalLink className="size-4" aria-hidden />
              Ver site de reservas
            </Link>
            <UserMenu userName={userName} roleLabel={roleLabel} />
          </div>
        </header>

        <main className="flex-1 px-4 py-6 md:px-6">
          <div className="mx-auto max-w-7xl">{children}</div>
        </main>

        <footer className="px-6 pb-6 text-center text-xs text-ink-500 md:text-left">
          InnKeep · sistema de gestão hoteleira de código aberto
        </footer>
      </div>
    </div>
  );
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/);
  const first = parts[0]?.[0] ?? "";
  const last = parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? "") : "";
  return (first + last).toUpperCase() || "?";
}

function UserMenu({ userName, roleLabel }: { userName: string; roleLabel: string }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // Fecha ao clicar fora ou apertar Esc. O listener só existe enquanto o menu
  // está aberto, e o retorno do efeito o remove (sem vazamento).
  useEffect(() => {
    if (!open) return;
    const onPointer = (event: PointerEvent) => {
      if (!ref.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        aria-haspopup="menu"
        className="flex items-center gap-2 rounded-full py-1 pr-2 pl-1 hover:bg-ink-100"
      >
        <span className="grid size-9 place-items-center rounded-full bg-brand-600 text-sm font-bold text-white">
          {initials(userName)}
        </span>
        <span className="hidden text-left leading-tight md:block">
          <span className="block max-w-40 truncate text-sm font-bold text-ink-900">{userName}</span>
          <span className="block text-xs text-ink-500">{roleLabel}</span>
        </span>
        <ChevronDown className="size-4 text-ink-500" aria-hidden />
      </button>

      {open ? (
        <div role="menu" className="absolute right-0 mt-2 w-56 rounded-md bg-white py-2 shadow-lg ring-1 ring-ink-200">
          <div className="border-b border-ink-100 px-4 pb-2 md:hidden">
            <p className="truncate text-sm font-bold text-ink-900">{userName}</p>
            <p className="text-xs text-ink-500">{roleLabel}</p>
          </div>
          <Link
            role="menuitem"
            href="/admin/users"
            onClick={() => setOpen(false)}
            className="flex items-center gap-2.5 px-4 py-2 text-sm font-semibold text-ink-700 hover:bg-brand-50 hover:text-brand-700"
          >
            <UserCog className="size-4" aria-hidden />
            Equipe e conta
          </Link>
          <form action={logout}>
            <button
              role="menuitem"
              type="submit"
              className="flex w-full items-center gap-2.5 px-4 py-2 text-sm font-semibold text-danger-ink hover:bg-danger-soft"
            >
              <LogOut className="size-4" aria-hidden />
              Sair
            </button>
          </form>
        </div>
      ) : null}
    </div>
  );
}
