import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("board shows columns with seeded issues", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1");
  // la board DEMO ha colonne dagli status del workflow; almeno una colonna visibile
  await expect(page.getByRole("heading", { name: /DEMO board/i })).toBeVisible();
  await expect(page.locator('[data-testid^="column-"]').first()).toBeVisible();
  // almeno una card issue del progetto DEMO
  await expect(page.locator('[data-testid^="card-DEMO-"]').first()).toBeVisible();
});

test("backlog page lists sprints controls and creates a sprint", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");
  await expect(page.getByRole("heading", { name: /Backlog/i })).toBeVisible();
  await page.getByLabel("New sprint name").fill("E2E Sprint");
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText("E2E Sprint")).toBeVisible();
  // lo sprint appena creato è "future" → mostra il bottone Start
  await expect(page.getByRole("button", { name: "Start sprint" }).first()).toBeVisible();
});

test("create board action on project overview unlocks board and backlog", async ({ page }) => {
  await login(page);

  // Il progetto DEMO seedato ha già una board: creiamo un progetto nuovo per
  // esercitare il percorso "nessuna board ancora" → Create board.
  await page.getByRole("button", { name: /create project/i }).click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: /select project type/i })).toBeVisible();
  await modal.getByRole("button", { name: "Next" }).click();
  await expect(modal.getByRole("heading", { name: "Create project" })).toBeVisible();
  await modal.locator('input[placeholder="My awesome project"]').fill("No Board Project");
  await modal.locator('input[placeholder="MAP"]').fill("NOBRD");
  await modal.getByRole("button", { name: "Create project" }).click();
  await expect(page.getByText("No Board Project")).toBeVisible();

  await page.locator('a[href="/app/projects/NOBRD"]').click();
  await page.waitForURL(/\/app\/projects\/NOBRD$/);

  const tabs = page.locator('[data-testid="project-overview-tabs"]');
  // Niente board ancora: Board/Backlog non sono link (disabled), il prompt di
  // creazione è visibile con il nome di default precompilato.
  await expect(tabs.getByText("Board")).toBeVisible();
  await expect(tabs.getByRole("link", { name: "Board" })).toHaveCount(0);
  await expect(page.getByLabel("Board name")).toHaveValue("No Board Project board");

  await page.getByRole("button", { name: "Create board" }).click();

  // Dopo la creazione, Board/Backlog si sbloccano e puntano alla nuova board.
  await expect(tabs.getByRole("link", { name: "Board" })).toBeVisible();
  const boardHref = await tabs.getByRole("link", { name: "Board" }).getAttribute("href");
  expect(boardHref).toMatch(/^\/app\/boards\/\d+$/);
  await expect(tabs.getByRole("link", { name: "Backlog" })).toHaveAttribute("href", `${boardHref}/backlog`);
});

test("create issue from backlog appears in the backlog list", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");

  await page.getByRole("button", { name: "Create issue" }).click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  // Contesto board/backlog: nessun project picker, il progetto è già fissato.
  await expect(modal.getByLabel("Project")).toHaveCount(0);
  await modal.locator("#issue-summary").fill("E2E Backlog Issue");
  await modal.getByRole("button", { name: "Create", exact: true }).click();

  await expect(modal).toHaveCount(0);
  await expect(page.getByText("E2E Backlog Issue")).toBeVisible();
});

test("topbar Create menu opens the issue modal with a project picker", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects");

  await page.getByRole("button", { name: "Create", exact: true }).click();
  await page.getByRole("button", { name: "Issue", exact: true }).click();

  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  await expect(modal.getByLabel("Project")).toBeVisible();
});
