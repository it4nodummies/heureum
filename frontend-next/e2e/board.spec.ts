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

async function dragBetween(page: Page, fromTestId: string, toTestId: string) {
  const source = page.getByTestId(fromTestId);
  const target = page.getByTestId(toTestId);
  // Source and target can both live in the same scrollable container (e.g.
  // BoardColumns.tsx's `overflow-x-auto` column row). Measuring both boxes up
  // front and only then moving the mouse is unsafe: scrolling the target into
  // view shifts that same container and invalidates the already-captured
  // source box, so the mouse never actually lands on the draggable element.
  // Instead, grab the source first (while its box is still valid) and only
  // scroll+remeasure the target right before moving to it — the virtual
  // mouse position isn't affected by the container's scroll.
  await source.scrollIntoViewIfNeeded();
  const sourceBox = await source.boundingBox();
  if (!sourceBox) throw new Error(`source ${fromTestId} not found`);
  await page.mouse.move(sourceBox.x + sourceBox.width / 2, sourceBox.y + sourceBox.height / 2);
  await page.mouse.down();
  await target.scrollIntoViewIfNeeded();
  const targetBox = await target.boundingBox();
  if (!targetBox) throw new Error(`target ${toTestId} not found`);
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

  await dragBetween(page, cardTestId, columnTestId);

  await expect(page.getByTestId("move-error")).toBeVisible();
  await expect(page.getByTestId("move-error")).toContainText("invalid transition");

  // Dismissing clears the banner.
  await page.getByRole("button", { name: "Dismiss error" }).click();
  await expect(page.getByTestId("move-error")).not.toBeVisible();
});

test("backlog: drag a single issue into a sprint", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");

  const sprintName = `Sprint DnD ${Date.now()}`;
  await page.getByLabel("New sprint name").fill(sprintName);
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText(sprintName)).toBeVisible();

  // The sprint's name lives in DroppableList's `header`, a DOM sibling of the inner droppable
  // div — not a descendant of it — so the lookup goes through the outer wrapper's
  // `container-{testId}` testid (see Task 1's note) to find the sprint by name, then strips the
  // prefix to get the real drop-target/containment testid.
  const sprintOuter = page.locator('[data-testid^="container-sprint-"]').filter({ hasText: sprintName });
  const outerTestId = await sprintOuter.getAttribute("data-testid");
  if (!outerTestId) throw new Error("sprint container testid not found");
  const sprintTestId = outerTestId.replace("container-", "");
  const sprintContainer = page.getByTestId(sprintTestId);

  await expect(page.getByTestId("row-DEMO-1")).toBeVisible();
  await dragBetween(page, "drag-handle-DEMO-1", sprintTestId);

  await expect(sprintContainer.getByTestId("row-DEMO-1")).toBeVisible();
  await expect(page.getByTestId("backlog-list").getByTestId("row-DEMO-1")).not.toBeVisible();
});

test("backlog: drag between two sprints directly, without going through the backlog", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");

  const sprintAName = `Sprint A ${Date.now()}`;
  await page.getByLabel("New sprint name").fill(sprintAName);
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText(sprintAName)).toBeVisible();

  const sprintBName = `Sprint B ${Date.now()}`;
  await page.getByLabel("New sprint name").fill(sprintBName);
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText(sprintBName)).toBeVisible();

  // See Task 3's comment: lookup by name goes through the outer `container-{testId}` wrapper,
  // then strips the prefix to get the real drop-target/containment testid.
  const sprintAOuter = page.locator('[data-testid^="container-sprint-"]').filter({ hasText: sprintAName });
  const sprintBOuter = page.locator('[data-testid^="container-sprint-"]').filter({ hasText: sprintBName });
  const sprintAOuterTestId = await sprintAOuter.getAttribute("data-testid");
  const sprintBOuterTestId = await sprintBOuter.getAttribute("data-testid");
  if (!sprintAOuterTestId || !sprintBOuterTestId) throw new Error("sprint container testid not found");
  const sprintATestId = sprintAOuterTestId.replace("container-", "");
  const sprintBTestId = sprintBOuterTestId.replace("container-", "");
  const sprintA = page.getByTestId(sprintATestId);
  const sprintB = page.getByTestId(sprintBTestId);

  await dragBetween(page, "drag-handle-DEMO-2", sprintATestId);
  await expect(sprintA.getByTestId("row-DEMO-2")).toBeVisible();
  // The optimistic local state that lands the row in Sprint A right after drop is briefly
  // superseded by a stale re-sync from the in-flight invalidate/refetch (see moveAndRank's
  // onSuccess in page.tsx) before settling back to the correct post-move data — i.e. the row can
  // flash out of Sprint A and back again a moment after this assertion first passes. Starting the
  // next drag on `drag-handle-DEMO-2` while that flash is mid-flight grabs a DOM node that gets
  // unmounted underneath the action (Playwright: "Element is not attached to the DOM"), since
  // moving an item across containers unmounts/remounts it rather than just repositioning it in
  // place. Waiting for the network to go idle here ensures the invalidated queries' refetch has
  // actually landed before the next drag starts.
  await page.waitForLoadState("networkidle");

  await dragBetween(page, "drag-handle-DEMO-2", sprintBTestId);
  await expect(sprintB.getByTestId("row-DEMO-2")).toBeVisible();
  await expect(sprintA.getByTestId("row-DEMO-2")).not.toBeVisible();
});

test("backlog: multi-select drag moves all selected issues together", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");

  const sprintName = `Sprint Group ${Date.now()}`;
  await page.getByLabel("New sprint name").fill(sprintName);
  await page.getByRole("button", { name: "Create sprint" }).click();
  await expect(page.getByText(sprintName)).toBeVisible();
  // See Task 3's comment: lookup by name goes through the outer `container-{testId}` wrapper.
  const sprintOuter = page.locator('[data-testid^="container-sprint-"]').filter({ hasText: sprintName });
  const outerTestId = await sprintOuter.getAttribute("data-testid");
  if (!outerTestId) throw new Error("sprint container testid not found");
  const sprintTestId = outerTestId.replace("container-", "");
  const sprintContainer = page.getByTestId(sprintTestId);

  await page.getByLabel("Select DEMO-4").check();
  await page.getByLabel("Select DEMO-5").check();

  await dragBetween(page, "drag-handle-DEMO-4", sprintTestId);

  await expect(sprintContainer.getByTestId("row-DEMO-4")).toBeVisible();
  await expect(sprintContainer.getByTestId("row-DEMO-5")).toBeVisible();
});
