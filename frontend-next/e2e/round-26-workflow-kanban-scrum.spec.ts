import { test, expect, type Page } from "@playwright/test";

// R26 acceptance: software (scrum/kanban) projects get a 4-state default
// workflow (TO DO → IN PROGRESS → TO BE TESTED → DONE), kanban boards get a
// FLAT backlog (no sprints) with a per-row "Send to board" action, and business
// projects stay on the lean 3-state workflow.

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

// Copied from board.spec.ts: a drag helper resilient to the container scrolling
// (grab the source first, only scroll+remeasure the target right before moving).
async function dragBetween(page: Page, fromTestId: string, toTestId: string) {
  const source = page.getByTestId(fromTestId);
  const target = page.getByTestId(toTestId);
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

// Create a project of the given template (CreateProjectModal: "Scrum" / "Kanban"
// / "Business") and resolve its auto-scaffolded board id from the Board tab href
// (R25 zero-config: every new project already has a board — no "Create board"
// step). Returns the board id and the project key.
async function createProject(
  page: Page,
  template: "Scrum" | "Kanban" | "Business",
): Promise<{ boardId: number; key: string }> {
  // slice(-7) keeps the key ≤ 8 chars (letter + 7 digits) and unique enough
  // across the reused webServer/DB (reuseExistingServer) and parallel spec files.
  const stamp = Date.now().toString().slice(-7);
  const name = `${template} R26 ${stamp}`;
  const key = `${template[0]}${stamp}`;

  await page.getByRole("button", { name: /create project/i }).click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: /select project type/i })).toBeVisible();
  // Template labels ("Scrum"/"Kanban"/"Business") are distinct, non-overlapping
  // words, so a name regex targets exactly one template button.
  await modal.getByRole("button", { name: new RegExp(template) }).click();
  await modal.getByRole("button", { name: "Next" }).click();
  await expect(modal.getByRole("heading", { name: "Create project" })).toBeVisible();
  await modal.locator('input[placeholder="My awesome project"]').fill(name);
  await modal.locator('input[placeholder="MAP"]').fill(key);
  await modal.getByRole("button", { name: "Create project" }).click();
  await expect(page.getByText(name)).toBeVisible();

  await page.locator(`a[href="/app/projects/${key}"]`).click();
  await page.waitForURL(new RegExp(`/app/projects/${key}$`));

  const tabs = page.locator('[data-testid="project-header-tabs"]');
  await expect(tabs.getByRole("link", { name: "Board" })).toBeVisible();
  const boardHref = await tabs.getByRole("link", { name: "Board" }).getAttribute("href");
  const m = boardHref?.match(/\/app\/boards\/(\d+)$/);
  if (!m) throw new Error(`could not resolve board id from href: ${boardHref}`);
  return { boardId: Number(m[1]), key };
}

// Create an issue via the shared ProjectHeader "Create issue" button (project is
// fixed here, so there is no project picker). The header invalidates the board
// and backlog queries on success, so the new issue surfaces without a reload.
async function createIssueViaHeader(page: Page, summary: string) {
  await page.getByRole("button", { name: "Create issue" }).click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  await modal.locator("#issue-summary").fill(summary);
  await modal.getByRole("button", { name: "Create", exact: true }).click();
  await expect(modal).toHaveCount(0);
}

test("TO BE TESTED flow (scrum)", async ({ page }) => {
  await login(page);
  const { boardId } = await createProject(page, "Scrum");

  // A software (scrum) project gets the 4-state default workflow, so its board
  // has four columns including a "TO BE TESTED" QA column.
  await page.goto(`/app/boards/${boardId}`);
  await expect(page.locator('[data-testid^="column-"]').first()).toBeVisible();
  await expect(page.locator('[data-testid^="column-"]')).toHaveCount(4);
  await expect(page.getByTestId("column-TO BE TESTED")).toBeVisible();

  // A new issue lands in the initial TO DO column.
  const summary = `TBT flow ${Date.now()}`;
  await createIssueViaHeader(page, summary);
  const card = page.getByTestId("column-TO DO").locator('[data-testid^="card-"]').filter({ hasText: summary });
  await expect(card).toBeVisible();
  const cardTestId = await card.getAttribute("data-testid");
  if (!cardTestId) throw new Error("created card not found in TO DO column");
  const cardKey = cardTestId.replace("card-", "");

  // Walk the issue through the whole workflow, asserting it lands in each column.
  // TO DO → IN PROGRESS (Start Progress).
  await dragBetween(page, cardTestId, "column-IN PROGRESS");
  await expect(page.getByTestId("move-error")).not.toBeVisible();
  await expect(page.getByTestId("column-IN PROGRESS").getByTestId(`card-${cardKey}`)).toBeVisible();
  await expect(page.getByTestId("column-TO DO").getByTestId(`card-${cardKey}`)).toHaveCount(0);
  await page.waitForLoadState("networkidle");

  // IN PROGRESS → TO BE TESTED (Ready to Test).
  await dragBetween(page, cardTestId, "column-TO BE TESTED");
  await expect(page.getByTestId("move-error")).not.toBeVisible();
  await expect(page.getByTestId("column-TO BE TESTED").getByTestId(`card-${cardKey}`)).toBeVisible();
  await expect(page.getByTestId("column-IN PROGRESS").getByTestId(`card-${cardKey}`)).toHaveCount(0);
  await page.waitForLoadState("networkidle");

  // TO BE TESTED → DONE (Done).
  await dragBetween(page, cardTestId, "column-DONE");
  await expect(page.getByTestId("move-error")).not.toBeVisible();
  await expect(page.getByTestId("column-DONE").getByTestId(`card-${cardKey}`)).toBeVisible();
  await expect(page.getByTestId("column-TO BE TESTED").getByTestId(`card-${cardKey}`)).toHaveCount(0);
});

test("kanban flat backlog, no sprints", async ({ page }) => {
  await login(page);
  const { boardId } = await createProject(page, "Kanban");

  await page.goto(`/app/boards/${boardId}/backlog`);

  // The flat backlog renders its single rank-ordered list container.
  const backlogList = page.getByTestId("backlog-list");
  await expect(backlogList).toBeVisible();

  // A new (to-do) issue appears as a row in the flat backlog.
  const summary = `Kanban backlog ${Date.now()}`;
  await createIssueViaHeader(page, summary);
  const row = backlogList.locator('[data-testid^="row-"]').filter({ hasText: summary });
  await expect(row).toBeVisible();
  const rowTestId = await row.getAttribute("data-testid");
  if (!rowTestId) throw new Error("created backlog row not found");
  const key = rowTestId.replace("row-", "");

  // No sprints on a kanban board: no sprint sections and no create-sprint control.
  await expect(page.locator('[data-testid^="container-sprint-"]')).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Create sprint" })).toHaveCount(0);
  await expect(page.getByLabel("New sprint name")).toHaveCount(0);

  // "Send to board" transitions the issue to the first in-progress status, so it
  // leaves the backlog and shows up on the board.
  await page.getByTestId(`send-to-board-${key}`).click();
  await expect(backlogList.getByTestId(`row-${key}`)).toHaveCount(0);

  await page.goto(`/app/boards/${boardId}`);
  await expect(page.getByTestId(`card-${key}`)).toBeVisible();
});

test("business stays lean (3 columns)", async ({ page }) => {
  await login(page);
  const { boardId } = await createProject(page, "Business");

  // A business project keeps the classic 3-state workflow — three columns, and
  // no "TO BE TESTED" QA column.
  await page.goto(`/app/boards/${boardId}`);
  await expect(page.locator('[data-testid^="column-"]').first()).toBeVisible();
  await expect(page.locator('[data-testid^="column-"]')).toHaveCount(3);
  await expect(page.getByTestId("column-TO BE TESTED")).toHaveCount(0);
});
