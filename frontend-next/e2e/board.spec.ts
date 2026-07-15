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
  // Board id 1 è la board DEMO seedata (progetto "Demo Project"): la pagina
  // condivide il ProjectHeader, che mostra il nome del progetto (non più il
  // nome della board) e la tab "Board" marcata come attiva.
  await expect(page.getByRole("heading", { name: /Demo Project/ })).toBeVisible();
  const tabs = page.locator('[data-testid="project-header-tabs"]');
  await expect(tabs.getByRole("link", { name: "Board" })).toHaveAttribute("aria-current", "page");
  // la board DEMO ha colonne dagli status del workflow; almeno una colonna visibile
  await expect(page.locator('[data-testid^="column-"]').first()).toBeVisible();
  // almeno una card issue del progetto DEMO
  await expect(page.locator('[data-testid^="card-DEMO-"]').first()).toBeVisible();
});

test("backlog page lists sprints controls and creates a sprint", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");
  await expect(page.getByRole("heading", { name: /Demo Project/ })).toBeVisible();
  const tabs = page.locator('[data-testid="project-header-tabs"]');
  await expect(tabs.getByRole("link", { name: "Backlog" })).toHaveAttribute("aria-current", "page");
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

  const tabs = page.locator('[data-testid="project-header-tabs"]');
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

test("shared project header shows on board, backlog, reports and settings with the correct active tab", async ({ page }) => {
  await login(page);

  // Board id 1 = board della DEMO seedata. Ogni pagina di sezione del
  // progetto (board/backlog/reports/settings) monta lo stesso ProjectHeader:
  // stesso nome progetto + stesse 4 tab, con la tab corrente marcata via
  // aria-current="page" (vedi ProjectHeader.tsx).
  await page.goto("/app/boards/1");
  let tabs = page.locator('[data-testid="project-header-tabs"]');
  await expect(page.getByRole("heading", { name: /Demo Project/ })).toBeVisible();
  await expect(tabs.getByRole("link", { name: "Board" })).toHaveAttribute("aria-current", "page");
  await expect(tabs.getByRole("link", { name: "Backlog" })).not.toHaveAttribute("aria-current", "page");
  await expect(tabs.getByRole("link", { name: "Reports" })).toBeVisible();
  await expect(tabs.getByRole("link", { name: "Settings" })).toBeVisible();

  await page.goto("/app/boards/1/backlog");
  tabs = page.locator('[data-testid="project-header-tabs"]');
  await expect(page.getByRole("heading", { name: /Demo Project/ })).toBeVisible();
  await expect(tabs.getByRole("link", { name: "Backlog" })).toHaveAttribute("aria-current", "page");
  await expect(tabs.getByRole("link", { name: "Board" })).not.toHaveAttribute("aria-current", "page");

  await page.goto("/app/projects/DEMO/reports");
  tabs = page.locator('[data-testid="project-header-tabs"]');
  await expect(page.getByRole("heading", { name: /Demo Project/ })).toBeVisible();
  await expect(tabs.getByRole("link", { name: "Reports" })).toHaveAttribute("aria-current", "page");
  await expect(tabs.getByRole("link", { name: "Board" })).toHaveAttribute("href", "/app/boards/1");
  await expect(tabs.getByRole("link", { name: "Backlog" })).toHaveAttribute("href", "/app/boards/1/backlog");

  await page.goto("/app/projects/DEMO/settings");
  tabs = page.locator('[data-testid="project-header-tabs"]');
  // Doppio heading atteso qui: il ProjectHeader condiviso ("Demo Project") e
  // il titolo interno di ProjectSettings ("Demo Project settings") — .first()
  // per evitare la strict-mode violation di Playwright.
  await expect(page.getByRole("heading", { name: /Demo Project/ }).first()).toBeVisible();
  await expect(tabs.getByRole("link", { name: "Settings" })).toHaveAttribute("aria-current", "page");
});

async function dragElement(page: Page, fromTestId: string, toTestId: string) {
  const source = page.getByTestId(fromTestId);
  const target = page.getByTestId(toTestId);
  const sourceBox = await source.boundingBox();
  const targetBox = await target.boundingBox();
  if (!sourceBox || !targetBox) throw new Error("drag element bounding box not found");
  await page.mouse.move(sourceBox.x + sourceBox.width / 2, sourceBox.y + sourceBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(targetBox.x + targetBox.width / 2, targetBox.y + targetBox.height / 2, { steps: 10 });
  await page.mouse.up();
}

test("dragging a card into a column with no valid transition shows an error", async ({ page }) => {
  await login(page);

  // Unique per run: the backend doesn't dedupe status names, and locally the
  // Playwright webServer is reused across invocations (reuseExistingServer),
  // so a fixed name would collide with a status left over from a prior run.
  const statusName = `Blocked-${Date.now()}`;

  // Add a status with no transitions to/from it via the Workflow settings panel.
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();
  await page.getByLabel("New status name").fill(statusName);
  await page.getByLabel("Category (reporting only)").selectOption("todo");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByTestId(`status-${statusName}`)).toBeVisible();

  // Go to the board: the new status appears as a column, but no transition
  // reaches it, so dropping a card there must fail visibly instead of silently.
  await page.goto("/app/boards/1");
  const columnTestId = `column-${statusName}`;
  await expect(page.locator(`[data-testid="${columnTestId}"]`)).toBeVisible();
  const card = page.locator('[data-testid^="card-DEMO-"]').first();
  const cardTestId = await card.getAttribute("data-testid");
  if (!cardTestId) throw new Error("no seeded card found on board 1");

  await dragElement(page, cardTestId, columnTestId);

  await expect(page.getByTestId("move-error")).toBeVisible();
  await expect(page.getByTestId("move-error")).toContainText("invalid transition");

  // Dismissing clears the banner.
  await page.getByRole("button", { name: "Dismiss error" }).click();
  await expect(page.getByTestId("move-error")).not.toBeVisible();
});
