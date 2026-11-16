import { BedDouble } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";

import { LoginForm } from "@/features/auth/login-form";
import { nunito } from "@/lib/fonts";

export const metadata: Metadata = { title: "Entrar" };

type Props = { searchParams: Promise<{ next?: string; expired?: string }> };

export default async function LoginPage({ searchParams }: Props) {
  const { next = "/admin", expired } = await searchParams;

  return (
    // Mesma identidade visual do painel (.admin-theme): é a porta de entrada dele.
    <main
      className={`${nunito.variable} admin-theme relative grid min-h-dvh place-items-center overflow-hidden px-4 py-12`}
    >
      {/* Manchas de cor desfocadas no fundo: só decoração, feitas em CSS, sem imagens. */}
      <div aria-hidden className="pointer-events-none absolute inset-0">
        <div className="absolute -top-32 -left-32 size-96 rounded-full bg-brand-300/40 blur-3xl" />
        <div className="absolute -right-24 -bottom-40 size-[28rem] rounded-full bg-accent/25 blur-3xl" />
      </div>

      <div className="relative w-full max-w-md">
        <div className="mb-6 flex items-center justify-center gap-2.5">
          <span className="grid size-10 place-items-center rounded-lg bg-brand-600 text-white shadow-[0_8px_16px_-8px] shadow-brand-600">
            <BedDouble className="size-5" aria-hidden />
          </span>
          <span className="text-2xl font-extrabold tracking-tight text-ink-900">InnKeep</span>
        </div>

        <div className="rounded-md bg-white/90 p-7 shadow-[0_10px_40px_-12px_rgba(67,97,238,0.35)] backdrop-blur sm:p-9">
          <h1 className="text-3xl font-extrabold tracking-tight text-brand-600 uppercase">Entrar</h1>
          <p className="mt-1 mb-6 text-sm font-semibold text-ink-500">Acesso da equipe: use seu e-mail e senha.</p>
          {expired ? (
            <p className="mb-4 rounded-md bg-warning-soft px-3 py-2 text-sm font-semibold text-warning-ink">
              Sua sessão expirou. Entre novamente.
            </p>
          ) : null}
          <LoginForm next={next} />
        </div>

        <p className="mt-6 text-center text-sm font-semibold text-ink-500">
          <Link href="/" className="underline-offset-4 hover:text-brand-700 hover:underline">
            Voltar ao site de reservas
          </Link>
        </p>
      </div>
    </main>
  );
}
