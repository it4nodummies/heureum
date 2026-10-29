import { test, expect, type Page } from "@playwright/test";

// Round 28 — epic experience: strict hierarchy Epic(1) → Story/Task/Bug(0) →
// Subtask(-1) within one project. Covers (a) creating an epic and a story
// under it from the create modal, (b) the backlog epic panel (filter, drag
// onto an epic, progress, inline create), (c) the issue-detail parent field
// (change / clear), the epic's child-issues progress and colour picker,
// (d) the board epic quick filter, (e) hierarchy violations through the REST
// API (400 with errors.parent).
//
// (a)–(d) share one scrum project created through the UI, so they run as one
// serial group; (e) is API-only and independent.

const API = process.env.E2E_API_URL ?? "http://localhost:8080";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

// Copied from round-26-workflow-kanban-scrum.spec.ts (itself from board.spec.ts):
// grab the source first, scroll + remeasure the target right before moving.
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

// Create a scrum project through CreateProjectModal and resolve its scaffolded
// board id from the Board tab (same flow as round-26). The key is a letter +
// 7 digits: unique enough across the shared e2e database.
async function createScrumProject(page: Page): Promise<{ boardId: number; key: string }> {
  const stamp = Date.now().toString().slice(-7);
  const name = `Epics R28 ${stamp}`;
  const key = `E${stamp}`;

  await page.getByRole("button", { name: /create project/i }).click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: /select project type/i })).toBeVisible();
  await modal.getByRole("button", { name: /Scrum/ }).click();
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

// Create an issue from the ProjectHeader "Create issue" modal (project fixed),
// optionally picking the type and the epic (the Epic select is only rendered
// for level-0 types). Returns nothing: the key is resolved from the backlog
// row afterwards (see backlogKeyOf).
async function createIssueViaHeader(page: Page, summary: string, opts: { type?: string; epicKey?: string } = {}) {
  await page.getByRole("button", { name: "Create issue" }).click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  if (opts.type) await modal.locator("#issue-type").selectOption(opts.type);
  if (opts.epicKey) await modal.getByTestId("create-modal-epic-select").selectOption(opts.epicKey);
  await modal.locator("#issue-summary").fill(summary);
  await modal.getByRole("button", { name: "Create", exact: true }).click();
  await expect(modal).toHaveCount(0);
}

// Key of the backlog row showing `summary` (the backlog page must be open).
async function backlogKeyOf(page: Page, summary: string): Promise<string> {
  const row = page.getByTestId("backlog-list").locator('[data-testid^="row-"]').filter({ hasText: summary });
  await expect(row).toBeVisible();
  const id = await row.getAttribute("data-testid");
  if (!id) throw new Error(`backlog row for "${summary}" not found`);
  return id.replace("row-", "");
}

test.describe.serial("epic experience", () => {
  let boardId: number;
  let projectKey: string;
  let epic1: string;
  let epic2: string;
  let story1: string; // created under epic1 from the modal
  let story2: string; // created without an epic, dragged onto epic1

  test("(a) create an epic and a story under it from the create modal", async ({ page }) => {
    test.setTimeout(60_000);
    await login(page);
    ({ boardId, key: projectKey } = await createScrumProject(page));
    await page.goto(`/app/boards/${boardId}/backlog`);
    await expect(page.getByTestId("backlog-list")).toBeVisible();

    await createIssueViaHeader(page, "Epic Alpha", { type: "Epic" });
    epic1 = await backlogKeyOf(page, "Epic Alpha");
    // The new epic is listed by the panel at once (no reload).
    await expect(page.getByTestId(`epic-panel-item-${epic1}`)).toBeVisible();

    await createIssueViaHeader(page, "Story under alpha", { type: "Story", epicKey: epic1 });
    story1 = await backlogKeyOf(page, "Story under alpha");

    // The backlog row carries the epic lozenge with the epic's name.
    await expect(page.getByTestId(`epic-lozenge-${story1}`)).toBeVisible();
    await expect(page.getByTestId(`epic-lozenge-${story1}`)).toHaveText("Epic Alpha");

    // The story's detail shows the Epic field pointing at the epic.
    await page.goto(`/app/browse/${story1}`);
    await expect(page.getByTestId("parent-field")).toHaveAttribute("data-parent-key", epic1);
    await expect(page.getByTestId("parent-value")).toContainText("Epic Alpha");
  });

  test("(b) backlog epic panel: filter, drag onto an epic, progress, inline create", async ({ page }) => {
    test.setTimeout(60_000);
    await login(page);
    await page.goto(`/app/boards/${boardId}/backlog`);
    await expect(page.getByTestId("backlog-list")).toBeVisible();

    // An orphan story (no epic).
    await createIssueViaHeader(page, "Orphan story", { type: "Story" });
    story2 = await backlogKeyOf(page, "Orphan story");
    await expect(page.getByTestId(`epic-lozenge-${story2}`)).toHaveCount(0);
    await expect(page.getByTestId(`epic-panel-progress-${epic1}`)).toHaveText("0/1");

    const list = page.getByTestId("backlog-list");
    // Filter by the epic: only its story (the epic row itself is hidden too).
    await page.getByTestId(`epic-panel-item-${epic1}`).click();
    await expect(page.getByTestId(`epic-panel-item-${epic1}`)).toHaveAttribute("aria-pressed", "true");
    await expect(list.getByTestId(`row-${story1}`)).toBeVisible();
    await expect(list.getByTestId(`row-${story2}`)).toHaveCount(0);
    await expect(list.getByTestId(`row-${epic1}`)).toHaveCount(0);

    // "Issues without epic": only the orphan.
    await page.getByTestId("epic-panel-item-none").click();
    await expect(page.getByTestId("epic-panel-item-none")).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByTestId(`epic-panel-item-${epic1}`)).toHaveAttribute("aria-pressed", "false");
    await expect(list.getByTestId(`row-${story2}`)).toBeVisible();
    await expect(list.getByTestId(`row-${story1}`)).toHaveCount(0);

    // "All issues" restores everything.
    await page.getByTestId("epic-panel-item-all").click();
    await expect(list.getByTestId(`row-${story1}`)).toBeVisible();
    await expect(list.getByTestId(`row-${story2}`)).toBeVisible();
    await expect(list.getByTestId(`row-${epic1}`)).toBeVisible();

    // Drag the orphan onto the epic: lozenge appears, progress 0/1 → 0/2.
    await dragBetween(page, `drag-handle-${story2}`, `epic-panel-item-${epic1}`);
    await expect(page.getByTestId("backlog-drag-error")).toHaveCount(0);
    await expect(page.getByTestId(`epic-lozenge-${story2}`)).toBeVisible();
    await expect(page.getByTestId(`epic-panel-progress-${epic1}`)).toHaveText("0/2");

    // Inline create from the panel adds a new epic row.
    await page.getByTestId("epic-panel-create").click();
    await page.getByTestId("epic-panel-create-input").fill("Epic Beta");
    await page.getByTestId("epic-panel-create-submit").click();
    const betaItem = page.locator('[data-testid^="epic-panel-item-"]').filter({ hasText: "Epic Beta" });
    await expect(betaItem).toBeVisible();
    const betaId = await betaItem.getAttribute("data-testid");
    if (!betaId) throw new Error("Epic Beta panel item not found");
    epic2 = betaId.replace("epic-panel-item-", "");
    await expect(page.getByTestId(`epic-panel-progress-${epic2}`)).toHaveText("0/0");
    // The new epic is a backlog row as well.
    await expect(list.getByTestId(`row-${epic2}`)).toBeVisible();
  });

  test("(c) issue detail: change and clear the parent; epic progress and colour", async ({ page }) => {
    test.setTimeout(60_000);
    await login(page);

    // The epic's detail counts both children.
    await page.goto(`/app/browse/${epic1}`);
    await expect(page.getByTestId("child-issues")).toBeVisible();
    await expect(page.getByTestId("child-issues-progress-bar")).toHaveAttribute("data-total", "2");
    await expect(page.getByTestId("child-issues-progress-bar")).toHaveAttribute("data-done", "0");

    // Move story1 to the second epic through the picker.
    await page.goto(`/app/browse/${story1}`);
    await expect(page.getByTestId("parent-field")).toHaveAttribute("data-parent-key", epic1);
    await page.getByTestId("parent-edit").click();
    await expect(page.getByTestId("parent-picker-input")).toBeVisible();
    await page.getByTestId(`parent-picker-option-${epic2}`).click();
    await expect(page.getByTestId("parent-field")).toHaveAttribute("data-parent-key", epic2);
    await expect(page.getByTestId("parent-value")).toContainText("Epic Beta");
    await expect(page.getByTestId("parent-error")).toHaveCount(0);

    // Clear it.
    await page.getByTestId("parent-clear").click();
    await expect(page.getByTestId("parent-field")).toHaveAttribute("data-parent-key", "");
    await expect(page.getByTestId("parent-value")).toHaveText("None");
    await page.reload();
    await expect(page.getByTestId("parent-field")).toHaveAttribute("data-parent-key", "");

    // epic1 is left with story2 only; pick a colour and check it persists.
    await page.goto(`/app/browse/${epic1}`);
    await expect(page.getByTestId("child-issues-progress-bar")).toHaveAttribute("data-total", "1");
    await expect(page.getByTestId("epic-color-picker")).toBeVisible();
    await page.getByTestId("epic-color-color_5").click();
    await expect(page.getByTestId("epic-color-color_5")).toHaveAttribute("data-selected", "true");
    await page.reload();
    await expect(page.getByTestId("epic-color-color_5")).toHaveAttribute("data-selected", "true");
    await expect(page.locator('[data-testid^="epic-color-"][data-selected="true"]')).toHaveCount(1);
  });

  test("(d) board epic quick filter", async ({ page }) => {
    await login(page);
    await page.goto(`/app/boards/${boardId}`);
    await expect(page.getByTestId(`card-${story1}`)).toBeVisible();
    await expect(page.getByTestId(`card-${story2}`)).toBeVisible();

    // story2 belongs to epic1 (dragged in (b)); story1 has no epic (cleared in (c)).
    const filter = page.getByTestId("board-epic-filter");
    await filter.selectOption(epic1);
    await expect(page.getByTestId(`card-${story2}`)).toBeVisible();
    await expect(page.getByTestId(`card-${story1}`)).toHaveCount(0);

    await filter.selectOption("none");
    await expect(page.getByTestId(`card-${story1}`)).toBeVisible();
    await expect(page.getByTestId(`card-${story2}`)).toHaveCount(0);

    await filter.selectOption("all");
    await expect(page.getByTestId(`card-${story1}`)).toBeVisible();
    await expect(page.getByTestId(`card-${story2}`)).toBeVisible();
  });
});

test("(e) hierarchy violations through the REST API are 400 errors.parent", async ({ request }) => {
  const loginRes = await request.post(`${API}/rest/api/3/auth/login`, {
    data: { email: "admin@example.com", password: "admin-demo-123" },
  });
  expect(loginRes.status()).toBe(200);
  const { token } = (await loginRes.json()) as { token: string };
  const headers = { Authorization: `Bearer ${token}` };

  const stamp = Date.now().toString().slice(-7);
  const keyA = `H${stamp}`;
  const keyB = `J${stamp}`;
  for (const [key, name] of [
    [keyA, `Hierarchy A ${stamp}`],
    [keyB, `Hierarchy B ${stamp}`],
  ]) {
    const res = await request.post(`${API}/rest/api/3/project`, {
      headers,
      data: {
        key,
        name,
        projectTypeKey: "software",
        projectTemplateKey: "com.pyxis.greenhopper.jira:gh-scrum-template",
      },
    });
    expect(res.status(), `create project ${key}`).toBe(201);
  }

  async function create(project: string, summary: string, type: string, parentKey?: string) {
    const fields: Record<string, unknown> = {
      project: { key: project },
      summary,
      issuetype: { name: type },
    };
    if (parentKey) fields.parent = { key: parentKey };
    const res = await request.post(`${API}/rest/api/3/issue`, { headers, data: { fields } });
    const body = (await res.json()) as { key?: string; errors?: Record<string, string> };
    return { status: res.status(), body };
  }
  async function mustCreate(project: string, summary: string, type: string, parentKey?: string) {
    const { status, body } = await create(project, summary, type, parentKey);
    expect(status, `${type} "${summary}"${parentKey ? ` under ${parentKey}` : ""}`).toBe(201);
    return body.key as string;
  }

  const epicA = await mustCreate(keyA, "Epic A", "Epic");
  const storyA = await mustCreate(keyA, "Story A", "Story");
  const epicB = await mustCreate(keyB, "Epic B", "Epic");

  // Valid: Story → Epic, Subtask → Story.
  await mustCreate(keyA, "Story under epic", "Story", epicA);
  await mustCreate(keyA, "Subtask under story", "Subtask", storyA);

  // Violations: each is 400 with errors.parent, nothing created.
  const violations: Array<[string, string, string, string]> = [
    ["story under a story", "Story", "Bad story", storyA],
    ["subtask under an epic", "Subtask", "Bad subtask", epicA],
    ["epic with a parent", "Epic", "Bad epic", epicA],
    ["parent from another project", "Story", "Cross project", epicB],
    ["unknown parent key", "Story", "Unknown parent", `${keyA}-99999`],
  ];
  for (const [what, type, summary, parentKey] of violations) {
    const { status, body } = await create(keyA, summary, type, parentKey);
    expect(status, what).toBe(400);
    expect(typeof body.errors?.parent, `${what}: errors.parent`).toBe("string");
    expect(body.errors?.parent?.length, `${what}: errors.parent non-empty`).toBeGreaterThan(0);
  }
});
