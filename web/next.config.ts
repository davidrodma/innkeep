import type { NextConfig } from "next";

// Cabeçalhos de segurança enviados em todas as respostas do site.
const securityHeaders = [
  // Impede que o site seja embutido em um <iframe> de terceiros (clickjacking).
  { key: "X-Frame-Options", value: "DENY" },
  // O navegador não tenta "adivinhar" o tipo de um arquivo.
  { key: "X-Content-Type-Options", value: "nosniff" },
  // Não envia a URL completa como referência ao navegar para outros sites.
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  // O site não usa câmera, microfone nem localização.
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  // Só aceita HTTPS por 2 anos (ignorado em http://localhost).
  { key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains" },
];

const nextConfig: NextConfig = {
  // "standalone" gera um servidor mínimo em .next/standalone, com apenas os
  // arquivos necessários: é o que a imagem Docker de produção copia.
  output: "standalone",
  poweredByHeader: false,
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

export default nextConfig;
