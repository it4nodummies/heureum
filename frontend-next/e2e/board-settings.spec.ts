import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

// Crea un progetto + board freschi e ne restituisce il boardId. La board DEMO
// (id 1) è condivisa con e2e/board.spec.ts, che fa affidamento sul fallback 1:1
// (nessuna colonna persistita, gli status nuovi diventano colonne). Persistere
// una board-config su board 1 romperebbe quei test, quindi qui lavoriamo su una
// board isolata. Un nuovo progetto riceve il workflow di default (TO DO / IN
// PROGRESS / DONE), così la pagina impostazioni elenca 3 colonne di fallback.
async function createProjectWithBoard(page: Page): Promise<number> {
  const stamp = Date.now().toString().slice(-6);
  const name = `Board Settings ${stamp}`;
  const key = `S${stamp}`; // 2-10 char, inizia con lettera

  await page.getByRole("button", { name: /create project/i }).click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: /select project type/i })).toBeVisible();
  await modal.getByRole("button", { name: "Next" }).click();
  await expect(modal.getByRole("heading", { name: "Create project" })).toBeVisible();
  await modal.locator('input[placeholder="My awesome project"]').fill(name);
  await modal.locator('input[placeholder="MAP"]').fill(key);
  await modal.getByRole("button", { name: "Create project" }).click();
  await expect(page.getByText(name)).toBeVisible();

  await page.locator(`a[href="/app/projects/${key}"]`).click();
  await page.waitForURL(new RegExp(`/app/projects/${key}$`));

  await page.getByRole("button", { name: "Create board" }).click();
  const tabs = page.locator('[data-testid="project-header-tabs"]');
  await expect(tabs.getByRole("link", { name: "Board" })).toBeVisible();
  const boardHref = await tabs.getByRole("link", { name: "Board" }).getAttribute("href");
  const m = boardHref?.match(/\/app\/boards\/(\d+)$/);
  if (!m) throw new Error(`could not resolve board id from href: ${boardHref}`);
  return Number(m[1]);
}

test("board settings lists columns and persists a rename", async ({ page }) => {
  await login(page);
  const boardId = await createProjectWithBoard(page);

  // La pagina impostazioni carica la config (fallback 1:1 dagli status del
  // workflow di default per una board appena creata).
  await page.goto(`/app/boards/${boardId}/settings`);
  await expect(page.getByTestId("board-settings")).toBeVisible();

  // Le colonne configurate sono elencate.
  const firstColumn = page.locator('[data-testid^="settings-column-"]').first();
  await expect(firstColumn).toBeVisible();

  // Rinomina la prima colonna con un nome univoco per run.
  const newName = `Renamed ${Date.now()}`;
  await firstColumn.getByLabel("Column name").fill(newName);

  await page.getByRole("button", { name: "Save board settings" }).click();
  await expect(page.getByTestId("board-settings-saved")).toBeVisible();

  // Reload → la rinomina è persistita.
  await page.reload();
  await expect(page.getByTestId("board-settings")).toBeVisible();
  await expect(
    page.locator('[data-testid^="settings-column-"]').first().getByLabel("Column name"),
  ).toHaveValue(newName);
});
