import { test, expect, type APIRequestContext, type Page } from "@playwright/test";

// Round 29 — Italian search, filters, dashboards and teams (groups) pages.
// Chrome is translated (column headers, bulk bar, pagination, saved-filter
// sharing labels, gadget titles, empty states); data stays as stored (JQL text,
// issue summaries, status/priority names, user-named filters/dashboards/teams).
//
// The suite shares ONE database (workers: 1): the user whose locale this spec
// changes is restored to "" (unset) in afterAll. Fixtures live in a dedicated
// project, so DEMO's issue count (search specs assume < 25) is untouched.

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

// Italian short date, e.g. "3 nov 2026" (Intl, same options as useFormat().date).
const IT_DATE = /(?<!\d)\d{1,2} (gen|feb|mar|apr|mag|giu|lug|ago|set|ott|nov|dic) \d{4}(?!\d)/;

const stamp = Date.now().toString().slice(-7);
const projectKey = `S${stamp}`;
const jql = `project = ${projectKey} ORDER BY created ASC`;

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  const headers = await authHeaders(request);

  const project = await request.post(`${API}/rest/api/3/project`, {
    headers,
    data: {
      key: projectKey,
      name: `I18N ricerca ${stamp}`,
      projectTypeKey: "software",
      projectTemplateKey: "com.pyxis.greenhopper.jira:gh-scrum-template",
    },
  });
  expect(project.status(), `create project ${projectKey}`).toBe(201);

  for (const summary of ["Primo ticket di ricerca", "Secondo ticket di ricerca"]) {
    const created = await request.post(`${API}/rest/api/3/issue`, {
      headers,
      data: { fields: { project: { key: projectKey }, summary, issuetype: { name: "Task" } } },
    });
    expect(created.status(), `create issue ${summary}`).toBe(201);
  }

  await setSavedLocale(request, "it");
  await request.dispose();
});

test.afterAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  await setSavedLocale(request, "");
  await request.dispose();
});

test("search results and filters in Italian; JQL stays as typed", async ({ page }) => {
  await login(page);

  // Global search in the TopBar.
  const global = page.getByRole("banner").getByLabel("Cerca", { exact: true });
  await expect(global).toBeVisible();
  await expect(global).toHaveAttribute("placeholder", "Cerca");

  await page.goto(`/app/filters?jql=${encodeURIComponent(jql)}`);
  await expect(page.locator("html")).toHaveAttribute("lang", "it");
  await expect(page.getByRole("heading", { name: "Filtri", exact: true })).toBeVisible();
  // JQL text stays as typed.
  await expect(page.getByLabel("JQL")).toHaveValue(jql);
  await expect(page.getByRole("button", { name: "Cerca", exact: true })).toBeVisible();

  // Column headers (chrome) translated; values (data) as stored.
  for (const h of ["Chiave", "Riepilogo", "Stato", "Priorità", "Assegnatario", "Aggiornato"]) {
    await expect(page.getByRole("columnheader", { name: h, exact: true })).toBeVisible();
  }
  for (const h of ["Summary", "Status", "Priority", "Assignee", "Updated"]) {
    await expect(page.getByRole("columnheader", { name: h, exact: true })).toHaveCount(0);
  }
  await expect(page.getByLabel("Colonne").getByLabel("Priorità", { exact: true })).toBeChecked();
  await expect(page.getByRole("cell", { name: "Primo ticket di ricerca" })).toBeVisible();
  await expect(page.getByTestId("cell-assignee").first()).toHaveText("Non assegnato");
  const firstRow = page.getByRole("row").filter({ has: page.getByRole("cell", { name: "Primo ticket di ricerca" }) });
  await expect(firstRow).toContainText(IT_DATE);

  // Count + pagination.
  await expect(page.getByTestId("list-count")).toHaveText("2 risultati");
  await expect(page.getByTestId("page-prev")).toHaveText("Precedente");
  await expect(page.getByTestId("page-next")).toHaveText("Successiva");

  // Bulk actions.
  await page.getByLabel("Seleziona tutti").check();
  const bar = page.getByTestId("bulk-bar");
  await expect(bar).toContainText("2 selezionati");
  await expect(bar.getByLabel("Priorità in blocco")).toBeVisible();
  await expect(bar.getByRole("button", { name: "Assegnatario in blocco" })).toHaveText(/Assegnatario…/);
  await expect(bar.getByLabel("Imposta etichette")).toHaveAttribute(
    "placeholder",
    "Imposta etichette, separate da virgola",
  );
  await expect(bar.getByRole("button", { name: "Applica", exact: true })).toBeVisible();
  await expect(bar.getByRole("button", { name: "Elimina", exact: true })).toBeVisible();
  await bar.getByRole("button", { name: "Annulla selezione", exact: true }).click();
  await expect(bar).toHaveCount(0);

  // Save a shared filter: labels, badge and row actions in Italian; the
  // filter's own name (data) as typed.
  const filterName = `Filtro i18n ${stamp}`;
  await expect(page.getByRole("heading", { name: "Filtri salvati" })).toBeVisible();
  await page.getByLabel("Nome filtro").fill(filterName);
  await page.getByLabel("Condividi con il team").check();
  await page.getByRole("button", { name: "Salva filtro", exact: true }).click();
  const row = page.locator("li").filter({ has: page.getByRole("button", { name: filterName }) });
  await expect(row.getByTestId("filter-shared-badge")).toHaveText("Condiviso");
  await expect(row.getByTestId("filter-shared-badge")).toHaveAttribute("title", "Condiviso con il team");
  await expect(row.getByRole("button", { name: "Modifica filtro" })).toHaveText("Modifica");
  await row.getByRole("button", { name: "Modifica filtro" }).click();
  await expect(page.getByLabel("Modifica nome filtro")).toHaveValue(filterName);
  await page.getByRole("button", { name: "Annulla", exact: true }).click();
  await row.getByRole("button", { name: "Elimina filtro" }).click();
  await expect(page.getByRole("button", { name: filterName })).toHaveCount(0);

  // Invalid JQL → Italian client error.
  await page.getByLabel("JQL").fill("project = = =");
  await page.getByRole("button", { name: "Cerca", exact: true }).click();
  await expect(page.getByText("Query JQL non valida")).toBeVisible();
});

test("dashboards list and detail in Italian (gadget titles are chrome)", async ({ page }) => {
  await login(page);
  await page.goto("/app/dashboards");
  await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();

  const dashName = `Cruscotto ${stamp}`;
  await expect(page.getByLabel("Nome della nuova dashboard")).toHaveAttribute("placeholder", "Nome dashboard");
  await page.getByLabel("Nome della nuova dashboard").fill(dashName);
  await page.getByRole("main").getByRole("button", { name: "Crea", exact: true }).click();
  await page.getByRole("link", { name: dashName }).click();

  await expect(page.getByRole("heading", { name: dashName })).toBeVisible();
  await expect(page.getByText("Nessun gadget")).toBeVisible();
  const type = page.getByLabel("Tipo di gadget");
  await expect(type.locator("option")).toHaveText(["Assegnati a me", "Attività recenti"]);

  await type.selectOption("assigned_to_me");
  await page.getByRole("button", { name: "Aggiungi gadget" }).click();
  await expect(page.getByTestId("gadget")).toHaveCount(1);
  await type.selectOption("activity_stream");
  await page.getByRole("button", { name: "Aggiungi gadget" }).click();
  await expect(page.getByTestId("gadget")).toHaveCount(2);

  const gadgets = page.getByTestId("dashboard-gadgets");
  await expect(gadgets.getByRole("heading", { name: "Assegnati a me" })).toBeVisible();
  await expect(gadgets.getByRole("heading", { name: "Attività recenti" })).toBeVisible();
  await expect(gadgets.getByRole("heading", { name: "Assigned to me" })).toHaveCount(0);
  await expect(page.getByTestId("gadget").first().getByRole("button", { name: "Rimuovi" })).toBeVisible();
});

test("teams page in Italian", async ({ page }) => {
  await login(page);
  await page.goto("/app/groups");
  await expect(page.getByRole("heading", { name: "Team", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Crea team" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Tutti i team" })).toBeVisible();

  const team = `squadra-i18n-${stamp}`;
  await page.getByLabel("Nome del team").fill(team);
  await page.getByRole("button", { name: "Crea team" }).click();
  const list = page.getByTestId("groups-list");
  await expect(list.getByText(team, { exact: true })).toBeVisible();

  await list.getByText(team, { exact: true }).click();
  await expect(page.getByText("Nessun membro")).toBeVisible();
  await expect(page.getByLabel(`Aggiungi utente a ${team}`)).toHaveAttribute(
    "placeholder",
    "Cerca persone da aggiungere…",
  );

  let dialogMessage = "";
  page.once("dialog", (d) => {
    dialogMessage = d.message();
    void d.accept();
  });
  await list.getByRole("button", { name: `Elimina ${team}` }).click();
  await expect(list.getByText(team, { exact: true })).toHaveCount(0);
  expect(dialogMessage).toBe(`Eliminare il team "${team}"?`);
});
