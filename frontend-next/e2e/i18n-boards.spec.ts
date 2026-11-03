import { test, expect, type APIRequestContext, type Page } from "@playwright/test";

// Round 29 — Italian boards, backlog (scrum + kanban), board settings and the
// workflow editor. The admin's saved locale is switched to "it" for this file
// and restored to "" (unset) in afterAll: the suite shares ONE database
// (workers: 1). Data (status/column/sprint/epic names) is never translated.

const API = process.env.E2E_API_URL ?? "http://localhost:8080";
const ADMIN = { email: "admin@example.com", password: "admin-demo-123" };

async function authHeaders(request: APIRequestContext) {
  const res = await request.post(`${API}/rest/api/3/auth/login`, { data: ADMIN });
  expect(res.status(), "login admin").toBe(200);
  const { token } = (await res.json()) as { token: string };
  return { Authorization: `Bearer ${token}` };
}

async function setSavedLocale(request: APIRequestContext, locale: string) {
  const res = await request.put(`${API}/rest/api/3/myself`, {
    headers: await authHeaders(request),
    data: { locale },
  });
  expect(res.status(), `PUT /myself locale=${locale}`).toBe(200);
}

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill(ADMIN.email);
  await page.getByLabel(/password/i).fill(ADMIN.password);
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

type Headers = Record<string, string>;

// Creates a scrum/kanban project through the API and returns its key and its
// auto-scaffolded board id (R25 zero-config).
async function createProject(request: APIRequestContext, headers: Headers, kind: "scrum" | "kanban") {
  const stamp = Date.now().toString().slice(-7);
  const key = `${kind === "scrum" ? "Q" : "W"}${stamp}`;
  const res = await request.post(`${API}/rest/api/3/project`, {
    headers,
    data: {
      key,
      name: `I18n ${kind} ${stamp}`,
      projectTypeKey: "software",
      projectTemplateKey: `com.pyxis.greenhopper.jira:gh-${kind}-template`,
    },
  });
  expect(res.status(), `create ${kind} project ${key}`).toBe(201);
  // The board list has no project filter: page through it and match the
  // board's location (same lookup as ProjectHeader).
  for (let startAt = 0; ; startAt += 50) {
    const page = await request.get(`${API}/rest/agile/1.0/board?startAt=${startAt}&maxResults=50`, { headers });
    expect(page.status()).toBe(200);
    const { values, isLast } = (await page.json()) as {
      values: { id: number; location?: { projectKey?: string } }[];
      isLast?: boolean;
    };
    const board = values.find((b) => b.location?.projectKey === key);
    if (board) return { key, boardId: board.id };
    if (isLast !== false || values.length === 0) throw new Error(`no board found for ${key}`);
  }
}

async function createIssue(request: APIRequestContext, headers: Headers, project: string, summary: string) {
  const res = await request.post(`${API}/rest/api/3/issue`, {
    headers,
    data: { fields: { project: { key: project }, summary, issuetype: { name: "Task" } } },
  });
  expect(res.status(), `create issue "${summary}"`).toBe(201);
  return ((await res.json()) as { key: string }).key;
}

test.describe.configure({ mode: "serial" });

let scrum: { key: string; boardId: number };
let kanban: { key: string; boardId: number };
let kanbanIssue: string;
let scrumIssues: string[];

test.beforeAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  const headers = await authHeaders(request);
  scrum = await createProject(request, headers, "scrum");
  kanban = await createProject(request, headers, "kanban");

  // A future sprint with dates and two open issues (→ plural on completion).
  const sprintRes = await request.post(`${API}/rest/agile/1.0/sprint`, {
    headers,
    data: { name: "Sprint I18n", originBoardId: scrum.boardId, startDate: "2026-11-03", endDate: "2026-11-17" },
  });
  expect(sprintRes.status(), "create sprint").toBe(201);
  const sprintId = ((await sprintRes.json()) as { id: number }).id;
  const a = await createIssue(request, headers, scrum.key, "Primo ticket");
  const b = await createIssue(request, headers, scrum.key, "Secondo ticket");
  const move = await request.post(`${API}/rest/agile/1.0/sprint/${sprintId}/issue`, {
    headers,
    data: { issues: [a, b] },
  });
  expect(move.status(), "move issues into sprint").toBeLessThan(300);
  scrumIssues = [a, b];

  kanbanIssue = await createIssue(request, headers, kanban.key, "Ticket kanban");

  await setSavedLocale(request, "it");
  await request.dispose();
});

test.afterAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  await setSavedLocale(request, "");
  await request.dispose();
});

test("scrum backlog: epic panel, sprint controls, Italian dates and plurals", async ({ page }) => {
  await login(page);
  await page.goto(`/app/boards/${scrum.boardId}/backlog`);

  // Epic panel.
  const panel = page.getByTestId("epic-panel");
  await expect(panel.getByText("Epic", { exact: true })).toBeVisible();
  await expect(page.getByTestId("epic-panel-toggle")).toHaveText("Nascondi epic");
  await expect(page.getByTestId("epic-panel-item-all")).toHaveText("Tutti i ticket");
  await expect(page.getByTestId("epic-panel-item-none")).toHaveText("Ticket senza epic");
  await expect(page.getByTestId("epic-panel-create")).toHaveText("Crea epic");
  await expect(panel.getByText("Ancora nessuna epic", { exact: true })).toBeVisible();

  // Sprint creation form + backlog list.
  await expect(page.getByRole("button", { name: "Crea sprint" })).toBeVisible();
  await expect(page.getByLabel("Nome del nuovo sprint")).toHaveAttribute("placeholder", "Nome dello sprint");
  await expect(page.getByLabel("Obiettivo dello sprint", { exact: true })).toHaveAttribute(
    "placeholder",
    "Obiettivo dello sprint (facoltativo)",
  );
  await expect(page.getByTestId("backlog-list")).toContainText("Il backlog è vuoto");

  // Sprint header: state and dates in Italian; the sprint name is data.
  await expect(page.getByText("Sprint I18n", { exact: false }).first()).toBeVisible();
  await expect(page.getByText("(futuro)")).toBeVisible();
  await expect(page.getByTestId("sprint-goal")).toContainText("3 nov 2026 → 17 nov 2026");
  await expect(page.getByRole("button", { name: "Modifica sprint" })).toBeVisible();
  await expect(page.getByLabel(`Seleziona ${scrumIssues[0]}`)).toBeVisible();
  await expect(page.getByLabel(`Trascina ${scrumIssues[0]}`)).toBeVisible();

  await page.getByRole("button", { name: "Avvia sprint" }).click();
  await page.getByRole("button", { name: "Completa sprint" }).click();
  const dialog = page.getByTestId("complete-sprint-dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("Completa Sprint I18n")).toBeVisible();
  await expect(page.getByTestId("incomplete-count")).toHaveText("2 ticket incompleti verranno spostati.");
  await expect(dialog.getByText("Sposta nel backlog")).toBeVisible();
  await expect(dialog.getByText("Sposta nello sprint")).toBeVisible();
  await dialog.getByRole("button", { name: "Annulla" }).click();
  await expect(dialog).toHaveCount(0);

  // Singular: with one issue sent back to the backlog, one incomplete issue is left.
  const back = await page.request.post(`${API}/rest/agile/1.0/backlog/issue`, {
    headers: await authHeaders(page.request),
    data: { issues: [scrumIssues[1]] },
  });
  expect(back.status(), "move issue to backlog").toBeLessThan(300);
  await page.reload();
  await page.getByRole("button", { name: "Completa sprint" }).click();
  await expect(page.getByTestId("incomplete-count")).toHaveText("1 ticket incompleto verrà spostato.");
});

test("kanban backlog, board header, board settings and swimlane bands", async ({ page }) => {
  await login(page);
  await page.goto(`/app/boards/${kanban.boardId}/backlog`);

  await expect(page.getByTestId("container-backlog-list")).toContainText("Backlog (1)");
  const send = page.getByTestId(`send-to-board-${kanbanIssue}`);
  await expect(send).toHaveText("Invia alla board");
  await send.click();
  await expect(page.getByTestId("backlog-list")).toContainText("Nessun ticket da fare nel backlog");

  // Board settings: Italian chrome, column/status names stay as data.
  await page.goto(`/app/boards/${kanban.boardId}/settings`);
  await expect(page.getByRole("heading", { name: "Impostazioni board" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Colonne" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Aggiungi colonna" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Filtri rapidi" })).toBeVisible();
  await expect(page.getByText("Ancora nessun filtro rapido.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Aggiungi filtro" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Torna alla board" })).toBeVisible();
  await expect(page.getByTestId("settings-column-0").getByText("TO DO", { exact: true })).toBeVisible();
  const swimlane = page.getByLabel("Swimlane");
  await expect(swimlane.locator("option")).toHaveText(["Nessuna", "Assegnatario", "Epic"]);
  await swimlane.selectOption("epic");
  await page.getByRole("button", { name: "Salva impostazioni board" }).click();
  await expect(page.getByTestId("board-settings-saved")).toHaveText("Salvato");

  // Board: header controls + the "no epic" band.
  await page.goto(`/app/boards/${kanban.boardId}`);
  await expect(page.getByTestId("board-settings-link")).toHaveText("Impostazioni board");
  const epicFilter = page.getByLabel("Filtro epic");
  await expect(epicFilter.locator("option").nth(0)).toHaveText("Tutte le epic");
  await expect(epicFilter.locator("option").nth(1)).toHaveText("Nessuna epic");
  await expect(page.getByTestId("swimlane-none")).toContainText("Nessuna epic (1)");
  await expect(page.getByTestId(`card-${kanbanIssue}`)).toBeVisible();
  await expect(page.getByTestId("column-IN PROGRESS")).toBeVisible();
});

test("workflow editor labels are Italian, status names are data", async ({ page }) => {
  await login(page);
  await page.goto(`/app/projects/${scrum.key}/settings`);
  await page.getByRole("button", { name: "Workflow" }).click();

  await expect(page.getByRole("heading", { name: "Stati" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Transizioni" })).toBeVisible();
  await expect(page.getByTestId("status-TO DO")).toContainText("(da fare)");
  await expect(page.getByRole("button", { name: "Aggiungi stato" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Aggiungi transizione" })).toBeVisible();
  await expect(page.getByLabel("Nome del nuovo stato")).toHaveAttribute("placeholder", "Nome dello stato");
  await expect(page.getByLabel("Categoria (solo per i report)").locator("option")).toHaveText([
    "Da fare",
    "In corso",
    "Completato",
  ]);
  await expect(page.getByLabel("Stato di partenza").locator("option").first()).toHaveText("Stato di partenza");
  await expect(page.getByLabel("Richiedi assegnatario", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Elimina lo stato TO DO" })).toHaveText("Rimuovi");
  await expect(page.getByRole("button", { name: "Trascina per riordinare TO DO" })).toBeVisible();
});
