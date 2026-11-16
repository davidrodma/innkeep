"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

/**
 * Recarrega os dados da página em intervalos, enquanto estiver montado.
 *
 * É como a interface convive com a CONSISTÊNCIA EVENTUAL do back-end: criar
 * uma reserva devolve "em processamento", e a confirmação chega instantes
 * depois, por eventos. Em vez de travar a tela esperando, a página mostra o
 * estado atual e este componente pede uma atualização a cada poucos segundos
 * — router.refresh() refaz só a renderização no servidor, sem perder o
 * estado dos formulários nem recarregar a página inteira.
 *
 * A página decide QUANDO renderizá-lo (ex.: só enquanto o status for
 * "REQUESTED"); assim a atualização para sozinha quando não é mais necessária.
 */
export function AutoRefresh({ intervalMs = 1500, maxAttempts = 40 }: { intervalMs?: number; maxAttempts?: number }) {
  const router = useRouter();

  useEffect(() => {
    let attempts = 0;
    const timer = setInterval(() => {
      attempts += 1;
      if (attempts > maxAttempts) {
        clearInterval(timer);
        return;
      }
      router.refresh();
    }, intervalMs);
    return () => clearInterval(timer);
  }, [router, intervalMs, maxAttempts]);

  return null;
}
