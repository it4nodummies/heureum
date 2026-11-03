import { test, expect, type APIRequestContext, type Page } from "@playwright/test";

// Round 29 — Italian projects area: projects list + "Crea progetto" modal,
// project header/overview, settings tabs (Accesso, Automazioni, Campi
// personalizzati, Integrazioni), releases, reports, calendar (month/weekday
// names from Intl, week starting on Monday) and timeline. Data (project and
// release names) stays as stored.
//
// The suite shares ONE database (workers: 1): fixtures live in a dedicated
// project (DEMO's issue count is relied upon elsewhere) and the admin's locale
// is restored to "" (unset) in afterAll.

const API = process.env.E2E_API_URL ?? "http://localhost:8080";
const ADMIN = { email: "admin@example.com", password: "admin-demo-123" };

async function authHeaders(request: APIRequestContext) {
  const res = await request.post(`${API}/rest/api/3/auth/login`, { data: ADMIN });
  expect(res.status(), "login").toBe(200);
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

const stamp = Date.now().toString().slice(-7);
const projectKey = `P${stamp}`;
const projectName = `Progetto I18N ${stamp}`;
const releaseName = `v-it-${stamp}`;

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  const headers = await authHeaders(request);

  const project = await request.post(`${API}/rest/api/3/project`, {
    headers,
    data: {
      key: projectKey,
      name: projectName,
      projectTypeKey: "software",
      projectTemplateKey: "com.pyxis.greenhopper.jira:gh-scrum-template",
    },
  });
  expect(project.status(), `create project ${projectKey}`).toBe(201);

  // A dated sprint (Jan–Feb 2026) on the project's board, so the timeline
  // ruler has month and quarter headers to localise.
  let boardId = 0;
  for (let startAt = 0; !boardId; startAt += 100) {
    const page = await request.get(`${API}/rest/agile/1.0/board?startAt=${startAt}&maxResults=100`, { headers });
    expect(page.status(), "list boards").toBe(200);
    const body = (await page.json()) as { values: { id: number; location?: { projectKey?: string } }[]; isLast?: boolean; total?: number };
    boardId = body.values.find((b) => b.location?.projectKey === projectKey)?.id ?? 0;
    if (!boardId && body.values.length === 0) throw new Error(`no board for ${projectKey}`);
  }
  const sprint = await request.post(`${API}/rest/agile/1.0/sprint`, {
    headers,
    data: { name: `Sprint I18N ${stamp}`, originBoardId: boardId, startDate: "2026-01-10", endDate: "2026-02-20" },
  });
  expect(sprint.status(), "create sprint").toBe(201);

  const version = await request.post(`${API}/rest/api/3/version`, {
    headers,
    data: { project: projectKey, name: releaseName, releaseDate: "2026-12-31" },
  });
  expect(version.status(), "create version").toBe(201);

  await setSavedLocale(request, "it");
  await request.dispose();
});

test.afterAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  await setSavedLocale(request, "");
  await request.dispose();
});

test("projects list and the create-project modal in Italian", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects");
  await expect(page.getByRole("heading", { name: "Progetti", exact: true })).toBeVisible();
  await expect(page.getByPlaceholder("Cerca progetti…")).toBeVisible();
  await expect(page.getByText(/^\d+ progett[oi]$/)).toBeVisible();
  await expect(page.getByText("Nome", { exact: true })).toBeVisible();
  await expect(page.getByText("Chiave", { exact: true })).toBeVisible();
  await expect(page.getByText("Responsabile", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Crea progetto" }).first().click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: "Seleziona il tipo di progetto" })).toBeVisible();
  // Methodology names stay (glossary), descriptions are translated.
  await expect(modal.getByText("Scrum", { exact: true })).toBeVisible();
  await expect(modal.getByText("Kanban", { exact: true })).toBeVisible();
  await expect(modal.getByText("Business", { exact: true })).toBeVisible();
  await expect(modal.getByText("Sprint, backlog, grafici burndown")).toBeVisible();
  await modal.getByRole("button", { name: "Avanti" }).click();
  await expect(modal.getByRole("heading", { name: "Crea progetto" })).toBeVisible();
  await expect(modal.getByText("Nome del progetto")).toBeVisible();
  await expect(modal.getByText("Chiave del progetto")).toBeVisible();
  await expect(modal.getByRole("button", { name: "← Indietro" })).toBeVisible();
});

test("project header and overview in Italian", async ({ page }) => {
  await login(page);
  await page.goto(`/app/projects/${projectKey}`);
  await expect(page.getByRole("heading", { name: projectName })).toBeVisible();
  const tabs = page.getByTestId("project-header-tabs");
  for (const name of ["Board", "Backlog", "Timeline", "Calendario", "Release", "Report", "Impostazioni"]) {
    await expect(tabs.getByRole("link", { name, exact: true })).toBeVisible();
  }
  await expect(page.getByRole("button", { name: "Crea ticket" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Ticket recenti" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Esporta CSV" })).toBeVisible();
});

test("project settings tabs in Italian", async ({ page }) => {
  await login(page);
  await page.goto(`/app/projects/${projectKey}/settings`);
  await expect(page.getByRole("heading", { name: `Impostazioni di ${projectName}` })).toBeVisible();
  for (const name of ["Generale", "Workflow", "Riepilogo", "Integrazioni", "Automazioni", "Campi personalizzati", "Accesso"]) {
    await expect(page.getByRole("button", { name, exact: true })).toBeVisible();
  }
  await expect(page.getByRole("button", { name: "Salva modifiche" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Archivia progetto" })).toBeVisible();

  await page.getByRole("button", { name: "Accesso", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Membri" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Aggiungi membro" })).toBeVisible();
  await expect(page.getByPlaceholder("Cerca persone da aggiungere…")).toBeVisible();
  // Role label translated, stored value untouched.
  const roleSelect = page.getByTestId("member-row").first().getByRole("combobox");
  await expect(roleSelect.locator("option")).toHaveText(["amministratore", "membro", "lettore"]);
  await expect(page.getByLabel("Ruolo del nuovo team").locator("option:checked")).toHaveText("membro");
  await expect(page.getByLabel("Ruolo del nuovo team")).toHaveValue("member");

  await page.getByRole("button", { name: "Automazioni", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Regole di automazione" })).toBeVisible();
  await expect(page.getByText("Nessuna regola di automazione")).toBeVisible();
  await page.getByRole("button", { name: "Nuova regola" }).click();
  await expect(page.getByLabel("Nome della regola")).toBeVisible();
  await expect(page.getByLabel("Evento").locator("option:checked")).toHaveText("Ticket creato");
  await expect(page.getByRole("button", { name: "Aggiungi condizione" })).toBeVisible();

  await page.getByRole("button", { name: "Campi personalizzati", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Nuovo campo" })).toBeVisible();
  await expect(page.getByLabel("Tipo di campo").locator("option:checked")).toHaveText("Testo");
  await expect(page.getByRole("button", { name: "Aggiungi campo" })).toBeVisible();

  await page.getByRole("button", { name: "Integrazioni", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Webhook in uscita" })).toBeVisible();
  await expect(page.getByText("Nessun webhook")).toBeVisible();
  await expect(page.getByRole("button", { name: "Aggiungi webhook" })).toBeVisible();
});

test("releases in Italian, with an Italian date", async ({ page }) => {
  await login(page);
  await page.goto(`/app/projects/${projectKey}/releases`);
  const main = page.getByTestId("releases-page");
  await expect(main.getByRole("heading", { name: "Release" })).toBeVisible();
  await expect(main.getByRole("button", { name: "Crea release" })).toBeVisible();
  await expect(main.getByLabel("Nome della release")).toBeVisible();
  const row = page.getByRole("row", { name: new RegExp(releaseName) });
  await expect(row.getByText("Non rilasciata")).toBeVisible();
  await expect(row).toContainText("31 dic 2026");
  await expect(main.getByRole("columnheader", { name: "Data di rilascio" })).toBeVisible();
});

test("reports in Italian (titles, empty states, pie fields)", async ({ page }) => {
  await login(page);
  await page.goto(`/app/projects/${projectKey}/reports`);
  for (const name of ["Burndown", "Burnup", "Velocità", "Flusso cumulativo", "Creati e risolti (ultimi 14 giorni)", "Ripartizione"]) {
    await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  }
  await expect(page.getByText("Nessuno sprint completato")).toBeVisible();
  const pie = page.getByLabel("Campo del grafico a torta");
  await expect(pie.locator("option:checked")).toHaveText("Stato");
  await expect(pie.locator("option")).toHaveText(["Stato", "Priorità", "Assegnatario", "Tipo"]);
  await expect(page.getByTestId("cfd-chart")).toContainText("Da fare");
});

function itMonthTitle(d: Date): string {
  const s = new Intl.DateTimeFormat("it", { month: "long", year: "numeric" }).format(d);
  return s.charAt(0).toUpperCase() + s.slice(1);
}

test("calendar month and weekday names from Intl, Monday first", async ({ page }) => {
  await login(page);
  await page.goto(`/app/projects/${projectKey}/calendar`);
  const now = new Date();
  const month = new Intl.DateTimeFormat("it", { month: "long", year: "numeric" }).format(now);
  const expected = month.charAt(0).toUpperCase() + month.slice(1);
  await expect(page.getByTestId("calendar-title")).toHaveText(expected);
  const headers = page.getByTestId("calendar-weekday");
  await expect(headers).toHaveCount(7);
  await expect(headers.first()).toHaveText("lun");
  await expect(headers.last()).toHaveText("dom");
  await expect(page.getByRole("button", { name: "Mese precedente" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Mese successivo" })).toBeVisible();

  // Walk to March 2026: 1 March 2026 is a Sunday, so with Monday first there
  // are 6 leading blanks and day 1 sits in the 7th column, under "dom".
  const title = page.getByTestId("calendar-title");
  const cursor = new Date(now.getFullYear(), now.getMonth(), 1);
  const target = new Date(2026, 2, 1);
  while (cursor.getTime() !== target.getTime()) {
    const back = cursor > target;
    cursor.setMonth(cursor.getMonth() + (back ? -1 : 1));
    await page.getByRole("button", { name: back ? "Mese precedente" : "Mese successivo" }).click();
    await expect(title).toHaveText(itMonthTitle(cursor));
  }
  await expect(title).toHaveText("Marzo 2026");
  await expect(page.getByTestId("calendar-blank")).toHaveCount(6);
  const grid = page.getByTestId("calendar-grid");
  // 7 weekday headers + 6 blanks → the 14th cell is day 1, in the "dom" column.
  const dayOne = grid.locator(":scope > :nth-child(14)");
  await expect(dayOne).toHaveAttribute("data-testid", "calendar-day");
  await expect(dayOne.locator("div").first()).toHaveText("1");
  await expect(grid.locator(":scope > :nth-child(7)")).toHaveText("dom");
});

test("timeline in Italian", async ({ page }) => {
  await login(page);
  await page.goto(`/app/projects/${projectKey}/timeline`);
  const zoom = page.getByLabel("Zoom della timeline");
  await expect(zoom.locator("option")).toHaveText(["Settimane", "Mesi", "Trimestri"]);
  await expect(page.getByText("Zoom", { exact: true })).toBeVisible();

  // Ruler headers come from the server in English; they are re-formatted.
  const chart = page.getByTestId("timeline-chart");
  await zoom.selectOption("months");
  await expect(chart).toContainText("gen 2026");
  await expect(chart).toContainText("feb 2026");
  await expect(chart).not.toContainText("Jan 2026");
  await zoom.selectOption("quarters");
  await expect(chart).toContainText("T1 2026");
  // The release (31 Dec 2026) stretches the range across the year.
  await expect(chart).toContainText("T2 2026");
  await expect(chart).not.toContainText("2026-Q");
  // Bar tooltip: the type is chrome too (sprint stays per the glossary).
  await expect(chart.getByTestId("timeline-bar").first()).toHaveAttribute("title", /· (sprint|epic|release)/);
});
