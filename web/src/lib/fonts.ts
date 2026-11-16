import { Nunito } from "next/font/google";

/**
 * Fonte do painel administrativo.
 *
 * O next/font baixa a fonte NA COMPILAÇÃO e a serve do próprio site: o
 * navegador do usuário não faz nenhuma requisição ao Google (melhor para a
 * privacidade e para o desempenho). A variável CSS é lida em `.admin-theme`.
 */
export const nunito = Nunito({
  subsets: ["latin"],
  variable: "--font-nunito",
  display: "swap",
});
