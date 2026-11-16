import { expect, test, type Page } from "@playwright/test";

// Fluxo completo, do site público ao painel administrativo:
//
//   hóspede busca e reserva no site ──► saga (reservation ⇄ inventory ⇄ billing)
//   ──► recepção vê a reserva, recebe o sinal com duas formas de pagamento
//   ──► evento "sinal quitado" ──► reserva confirmada.

const SLUG = process.env.E2E_PROPERTY_SLUG ?? "pousada-recanto-da-serra";
const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? "admin@innkeep.local";
const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? "innkeep-dev-admin";

/** Data "AAAA-MM-DD" daqui a `days` dias. Datas distantes evitam colidir com o seed. */
function inDays(days: number): string {
  const date = new Date();
  date.setDate(date.getDate() + days);
  return date.toISOString().slice(0, 10);
}

const PROPERTY_NAME = process.env.E2E_PROPERTY_NAME ?? "Pousada Recanto da Serra";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel("E-mail").fill(ADMIN_EMAIL);
  await page.getByLabel("Senha").fill(ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Entrar" }).click();
  await expect(page).toHaveURL(/\/admin$/);

  // Com mais de uma propriedade (os testes de API criam outras), o painel
  // mostra um seletor: escolhe a propriedade de demonstração.
  const selector = page.getByLabel("Propriedade");
  if (await selector.isVisible()) {
    const current = await selector.locator("option:checked").textContent();
    if (current?.trim() !== PROPERTY_NAME) {
      // A troca é uma Server Action (POST): espera a resposta, senão a próxima
      // navegação pode sair antes de o cookie da propriedade ser gravado.
      await Promise.all([
        page.waitForResponse((response) => response.request().method() === "POST" && "next-action" in response.request().headers()),
        selector.selectOption({ label: PROPERTY_NAME }),
      ]);
      await expect(page.getByText(`${PROPERTY_NAME} ·`)).toBeVisible();
    }
  }
}

test("painel exige login", async ({ page }) => {
  await page.goto("/admin/reservations");
  await expect(page).toHaveURL(/\/login\?next=%2Fadmin%2Freservations/);
  await expect(page.getByRole("button", { name: "Entrar" })).toBeVisible();
});

test("login com senha errada mostra o erro da API", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("E-mail").fill(ADMIN_EMAIL);
  await page.getByLabel("Senha").fill("senha-errada-123");
  await page.getByRole("button", { name: "Entrar" }).click();
  // Filtra pelo texto: o Next.js também tem um role="alert" (anunciador de rotas).
  await expect(page.getByRole("alert").filter({ hasText: "e-mail ou senha incorretos" })).toBeVisible();
});

test("hóspede reserva pelo site e a recepção confirma recebendo o sinal", async ({ page }) => {
  // Uma janela aleatória de datas no futuro, para o teste poder rodar várias vezes.
  const offset = 60 + Math.floor(Math.random() * 200);
  const checkIn = inDays(offset);
  const checkOut = inDays(offset + 2);
  const guestName = `Hóspede E2E ${Date.now()}`;
  const email = `e2e-${Date.now()}@example.com`;

  // 1. Busca de disponibilidade no site público.
  await page.goto(`/p/${SLUG}?check_in=${checkIn}&check_out=${checkOut}&adults=2&children=0`);
  await expect(page.getByRole("heading", { name: /2 noite\(s\)/ })).toBeVisible();
  await page.getByRole("link", { name: "Reservar" }).first().click();

  // 2. Formulário de reserva.
  await expect(page.getByRole("heading", { name: "Finalizar reserva" })).toBeVisible();
  await page.getByLabel("Nome completo").fill(guestName);
  await page.getByLabel("E-mail").fill(email);
  await page.getByLabel("Telefone / WhatsApp").fill("+55 11 98888-0000");
  await page.getByLabel(/Li e aceito a política/).check();
  await page.getByRole("button", { name: "Confirmar reserva" }).click();

  // 3. Página de acompanhamento: a saga termina sozinha (auto-atualização).
  await expect(page).toHaveURL(/\/booking\/IK-/);
  await expect(page.getByRole("heading", { name: "Quase lá: falta o sinal" })).toBeVisible();
  const code = (await page.locator("p.font-mono").first().textContent())?.trim() ?? "";
  expect(code).toMatch(/^IK-[A-Z0-9]{6}$/);

  // 4. Recepção encontra a reserva no painel.
  await login(page);
  await page.goto(`/admin/reservations?q=${code}`);
  await page.getByRole("link", { name: code }).click();
  await expect(page.getByRole("heading", { name: guestName })).toBeVisible();
  await expect(page.getByText("Aguardando sinal").first()).toBeVisible();

  // 5. Recebe o sinal dividido em Pix + dinheiro.
  await expect(page.getByRole("heading", { name: "Receber pagamento" })).toBeVisible();
  await page.getByRole("button", { name: /Preencher sinal/ }).click();
  const amounts = page.getByLabel("Valor (R$)", { exact: true });
  const depositText = await amounts.first().inputValue();
  const depositCents = Math.round(Number(depositText.replace(",", ".")) * 100);
  const half = Math.floor(depositCents / 2);
  const format = (cents: number) => (cents / 100).toFixed(2).replace(".", ",");

  await amounts.first().fill(format(half));
  await page.getByRole("button", { name: "Outra forma de pagamento" }).click();
  await page.getByLabel("Forma de pagamento", { exact: true }).nth(1).selectOption("CASH");
  await amounts.nth(1).fill(format(depositCents - half));
  await page.getByRole("button", { name: "Registrar pagamento" }).click();

  // 6. O financeiro publica "sinal quitado" e o serviço de reservas confirma.
  await expect(async () => {
    await page.reload();
    await expect(page.getByText("Confirmada").first()).toBeVisible({ timeout: 2_000 });
  }).toPass({ timeout: 30_000 });
  await expect(page.getByText("Reserva confirmada")).toBeVisible(); // linha do histórico (Event Store)
});

test("calendário mostra a grade de quartos", async ({ page }) => {
  await login(page);
  await page.getByRole("link", { name: "Calendário" }).click();
  await expect(page.getByRole("heading", { name: "Calendário" })).toBeVisible();
  await expect(page.getByText("Quarto com beliches")).toBeVisible();
});
