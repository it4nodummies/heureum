import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

// Board DEMO (id 1) è condivisa con e2e/board.spec.ts, che si appoggia al
// fallback 1:1 (nessuna colonna persistita). Persistere una config su board 1
// romperebbe quei test, quindi qui lavoriamo su una board fresca isolata. Un
// nuovo progetto riceve il workflow di default (TO DO ⇄ IN PROGRESS → DONE).
async function createProjectWithBoard(page: Page): Promise<{ boardId: number; key: string }> {
  const stamp = Date.now().toString().slice(-6);
  const name = `Board Render ${stamp}`;
  const key = `R${stamp}`; // 2-10 char, inizia con lettera

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
  return { boardId: Number(m[1]), key };
}

// Copiato da board.spec.ts: drag helper resiliente allo scroll del container.
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

async function createBacklogIssue(page: Page, summary: string) {
  await page.getByRole("button", { name: "Create issue" }).click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  await modal.locator("#issue-summary").fill(summary);
  await modal.getByRole("button", { name: "Create", exact: true }).click();
  await expect(modal).toHaveCount(0);
}

test("board renders a merged column (status set) holding issues from both statuses, plus swimlanes", async ({
  page,
}) => {
  await login(page);
  const { boardId } = await createProjectWithBoard(page);

  // Due issue fresche nel progetto: partono entrambe in TO DO (primo status).
  await page.goto(`/app/boards/${boardId}/backlog`);
  const sumTodo = `Stays TODO ${Date.now()}`;
  const sumProg = `Moves to progress ${Date.now()}`;
  await createBacklogIssue(page, sumTodo);
  await createBacklogIssue(page, sumProg);

  // Sul board (config 1:1 di default) entrambe stanno nella colonna TO DO.
  await page.goto(`/app/boards/${boardId}`);
  const todoCard = page.locator('[data-testid^="card-"]').filter({ hasText: sumProg });
  const stayCard = page.locator('[data-testid^="card-"]').filter({ hasText: sumTodo });
  await expect(todoCard).toBeVisible();
  await expect(stayCard).toBeVisible();

  const progCardTestId = await todoCard.getAttribute("data-testid");
  const stayCardTestId = await stayCard.getAttribute("data-testid");
  if (!progCardTestId || !stayCardTestId) throw new Error("board cards not found");
  const progKey = progCardTestId.replace("card-", "");
  const stayKey = stayCardTestId.replace("card-", "");

  // Trascina una delle due in IN PROGRESS (transizione valida di default).
  await dragBetween(page, progCardTestId, "column-IN PROGRESS");
  await expect(page.getByTestId("column-IN PROGRESS").getByTestId(progCardTestId)).toBeVisible();

  // Impostazioni: unisci TO DO + IN PROGRESS in una singola colonna.
  await page.goto(`/app/boards/${boardId}/settings`);
  await expect(page.getByTestId("board-settings")).toBeVisible();
  const col0 = page.getByTestId("settings-column-0");
  const mergedName = `Merged ${Date.now()}`;
  await col0.getByLabel("Column name").fill(mergedName);
  // Aggiungi lo status IN PROGRESS alla colonna 0 (che già ha TO DO).
  await col0.getByRole("checkbox", { name: "IN PROGRESS" }).check();
  // Rimuovi la colonna IN PROGRESS ormai ridondante.
  await page.getByRole("button", { name: "Remove column IN PROGRESS" }).click();
  await page.getByRole("button", { name: "Save board settings" }).click();
  await expect(page.getByTestId("board-settings-saved")).toBeVisible();

  // Board: la colonna unita esiste e contiene ENTRAMBE le issue (TO DO + IN PROGRESS).
  await page.goto(`/app/boards/${boardId}`);
  const merged = page.getByTestId(`column-${mergedName}`);
  await expect(merged).toBeVisible();
  await expect(merged.getByTestId(`card-${progKey}`)).toBeVisible();
  await expect(merged.getByTestId(`card-${stayKey}`)).toBeVisible();

  // Swimlanes: imposta raggruppamento per assignee. Le issue sono non assegnate
  // → una banda "no assignee" comunque presente.
  await page.goto(`/app/boards/${boardId}/settings`);
  await expect(page.getByTestId("board-settings")).toBeVisible();
  await page.getByLabel("Swimlane").selectOption("assignee");
  await page.getByRole("button", { name: "Save board settings" }).click();
  await expect(page.getByTestId("board-settings-saved")).toBeVisible();

  await page.goto(`/app/boards/${boardId}`);
  const band = page.locator('[data-testid^="swimlane-"]').first();
  await expect(band).toBeVisible();
  // La banda contiene la colonna unita con le card raggruppate.
  await expect(band.getByTestId(`card-${progKey}`)).toBeVisible();
  await expect(band.getByTestId(`card-${stayKey}`)).toBeVisible();
});
