"use client";

// Página de erro do painel. Em Next.js, error.tsx é um "error boundary": se
// algo falhar ao renderizar uma página do /admin, esta tela aparece no lugar
// — o menu lateral continua funcionando.

import { Button } from "@/components/ui/button";

export default function AdminError({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const forbidden = error.message.includes("permissão");
  return (
    <div className="mx-auto max-w-lg py-16 text-center">
      <h1 className="text-xl font-semibold text-ink-900">{forbidden ? "Acesso não permitido" : "Não foi possível carregar esta tela"}</h1>
      <p className="mt-2 text-sm text-ink-600">
        {forbidden
          ? "O seu papel não dá acesso a esta área. Fale com um administrador se precisar."
          : "Pode ser uma instabilidade momentânea em algum serviço. Tente novamente em instantes."}
      </p>
      {error.digest ? <p className="mt-2 font-mono text-xs text-ink-400">Código: {error.digest}</p> : null}
      <Button className="mt-6" onClick={reset}>
        Tentar novamente
      </Button>
    </div>
  );
}
