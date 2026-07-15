import { test, expect, Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("crea un nuovo progetto e lo vede nella lista", async ({ page }) => {
  await login(page);
  await page.getByRole("button", { name: /create project/i }).click();

  // Step 1: template picker. Scrum is selected by default, so we just advance.
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: /select project type/i })).toBeVisible();
  await modal.getByRole("button", { name: "Next" }).click();

  // Step 2: name/key fields have no <label for>, so we target by placeholder
  // (see CreateProjectModal.tsx: "My awesome project" / "MAP" placeholders).
  await expect(modal.getByRole("heading", { name: "Create project" })).toBeVisible();
  await modal.locator('input[placeholder="My awesome project"]').fill("Marketing Site");
  await modal.locator('input[placeholder="MAP"]').fill("MKT");
  await modal.getByRole("button", { name: "Create project" }).click();

  await expect(page.getByText("Marketing Site")).toBeVisible();
});

test("apre le impostazioni del progetto DEMO dal menu azioni e rinomina", async ({ page }) => {
  await login(page);

  // Scope to the DEMO row (grid row carries both "group" and "cursor-pointer"
  // classes, unique to data rows vs. the header row) to avoid ambiguity with
  // any other project created by other tests.
  const demoRow = page.locator("div.group.cursor-pointer", { hasText: "Demo Project" });
  await demoRow.getByRole("button", { name: /project actions/i }).click();
  await demoRow.getByRole("button", { name: /project settings/i }).click();

  await page.waitForURL(/\/app\/projects\/DEMO\/settings/);
  const nameInput = page.locator("#proj-name");
  await expect(nameInput).toHaveValue(/Demo Project/);
  await nameInput.fill("Demo Project Renamed");
  await page.getByRole("button", { name: /save changes/i }).click();
  await expect(page.getByText(/saved/i)).toBeVisible();
});

test("clicca la riga del progetto DEMO nella lista e arriva alla sua overview (non 404)", async ({ page }) => {
  await login(page);

  // La riga del progetto ora è cliccabile: il nome è un link diretto
  // verso /app/projects/{key} (vedi ProjectsPage.tsx). Selezioniamo per href
  // esatto invece che per testo, perché il nome può essere stato rinominato
  // da un altro test dello stesso file (DB e-2e condiviso per l'intera run).
  await page.locator('a[href="/app/projects/DEMO"]').click();

  await page.waitForURL(/\/app\/projects\/DEMO$/);

  // Header del progetto: nome (eventualmente rinominato) + key.
  await expect(page.getByRole("heading", { name: /Demo Project/ })).toBeVisible();
  await expect(page.getByText("DEMO", { exact: true })).toBeVisible();

  // Barra dei link di sezione: Board/Backlog risolte dalla board del progetto
  // DEMO (board id 1, seedata — vedi board.spec.ts), Reports e Settings sempre presenti.
  const tabs = page.locator('[data-testid="project-overview-tabs"]');
  await expect(tabs).toBeVisible();
  await expect(tabs.getByRole("link", { name: "Board" })).toHaveAttribute("href", "/app/boards/1");
  await expect(tabs.getByRole("link", { name: "Backlog" })).toHaveAttribute("href", "/app/boards/1/backlog");
  await expect(tabs.getByRole("link", { name: "Reports" })).toHaveAttribute("href", "/app/projects/DEMO/reports");
  await expect(tabs.getByRole("link", { name: "Settings" })).toHaveAttribute("href", "/app/projects/DEMO/settings");

  // Sezione issue recenti presente (niente 404, niente crash).
  await expect(page.getByRole("heading", { name: "Recent issues" })).toBeVisible();
});
