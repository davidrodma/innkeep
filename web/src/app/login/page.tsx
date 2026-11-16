import type { Metadata } from "next";
import Link from "next/link";

import { LoginForm } from "@/features/auth/login-form";

export const metadata: Metadata = { title: "Entrar" };

type Props = { searchParams: Promise<{ next?: string; expired?: string }> };

export default async function LoginPage({ searchParams }: Props) {
  const { next = "/admin", expired } = await searchParams;

  return (
    <main className="grid min-h-dvh place-items-center px-4 py-12">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <p className="text-2xl font-semibold tracking-tight text-brand-700">InnKeep</p>
          <p className="mt-1 text-sm text-ink-500">Acesso da equipe</p>
        </div>
        <div className="rounded-xl border border-ink-200 bg-white p-6">
          {expired ? (
            <p className="mb-4 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800">
              Sua sessão expirou. Entre novamente.
            </p>
          ) : null}
          <LoginForm next={next} />
        </div>
        <p className="mt-6 text-center text-sm text-ink-500">
          <Link href="/" className="underline-offset-4 hover:underline">
            Voltar ao site de reservas
          </Link>
        </p>
      </div>
    </main>
  );
}
