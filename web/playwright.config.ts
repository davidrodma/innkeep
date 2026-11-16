import { defineConfig, devices } from "@playwright/test";

// Testes de ponta a ponta no navegador. Exigem a stack completa no ar
// (`docker compose up -d` + `docker compose run --rm seed`) e exercitam o
// sistema como um usuário real: clicando, digitando, esperando.
//
//   npx playwright install chromium   (uma vez)
//   npm run test:e2e
export default defineConfig({
  testDir: "./e2e",
  // Os fluxos passam por eventos assíncronos (Kafka): damos folga.
  timeout: 90_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: process.env.E2E_WEB_URL ?? "http://localhost:3000",
    locale: "pt-BR",
    timezoneId: "America/Sao_Paulo",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
