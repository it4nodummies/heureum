import { test, expect, type APIRequestContext, type Page } from "@playwright/test";

// Round 29 — Italian issue area: issue detail (field labels, comments, history,
// dates), the "Crea ticket" modal. Data (summary, status/type/priority names,
// history field values) stays as stored.
//
// The suite shares ONE database (workers: 1): the user whose locale this spec
// changes is restored to "" (unset) in afterAll.

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

let issueKey = "";
const summary = `I18N dettaglio ${Date.now()}`;

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  const headers = await authHeaders(request);

  // A dedicated project (so DEMO's issue count, which other specs depend on,
  // is untouched) and issue: created with priority High (id 2), then lowered
  // to Low (id 4) so the History tab has a "priority" row; plus one comment.
  const stamp = Date.now().toString().slice(-7);
  const projectKey = `I${stamp}`;
  const project = await request.post(`${API}/rest/api/3/project`, {
    headers,
    data: {
      key: projectKey,
      name: `I18N ticket ${stamp}`,
      projectTypeKey: "software",
      projectTemplateKey: "com.pyxis.greenhopper.jira:gh-scrum-template",
    },
  });
  expect(project.status(), `create project ${projectKey}`).toBe(201);

  const created = await request.post(`${API}/rest/api/3/issue`, {
    headers,
    data: {
      fields: {
        project: { key: projectKey },
        summary,
        issuetype: { name: "Task" },
        priority: { id: "2" },
      },
    },
  });
  expect(created.status(), "create issue").toBe(201);
  issueKey = ((await created.json()) as { key: string }).key;

  const updated = await request.put(`${API}/rest/api/3/issue/${issueKey}`, {
    headers,
    data: { fields: { priority: { id: "4" } } },
  });
  expect(updated.status(), "update priority").toBe(204);

  const comment = await request.post(`${API}/rest/api/3/issue/${issueKey}/comment`, {
    headers,
    data: {
      body: {
        type: "doc",
        version: 1,
        content: [{ type: "paragraph", content: [{ type: "text", text: "Commento di prova" }] }],
      },
    },
  });
  expect(comment.status(), "add comment").toBe(201);

  await setSavedLocale(request, "it");
  await request.dispose();
});

test.afterAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  await setSavedLocale(request, "");
  await request.dispose();
});

test("issue detail in Italian: field labels, comments, history and an Italian date", async ({ page }) => {
  await login(page);
  await page.goto(`/app/browse/${issueKey}`);
  await expect(page.locator("html")).toHaveAttribute("lang", "it");
  await expect(page.getByRole("heading", { name: summary })).toBeVisible();

  const aside = page.locator("aside").filter({ has: page.getByTestId("field-assignee") });
  for (const label of ["Stato", "Tipo", "Priorità", "Assegnatario", "Segnalatore", "Etichette"]) {
    await expect(aside.getByText(label, { exact: true })).toBeVisible();
  }
  for (const label of ["Status", "Priority", "Assignee", "Reporter"]) {
    await expect(aside.getByText(label, { exact: true })).toHaveCount(0);
  }
  // Values are data (priority name) or translated chrome ("Non assegnato").
  await expect(page.getByTestId("field-priority")).toHaveText("Low");
  await expect(page.getByTestId("field-assignee")).toHaveText("Non assegnato");
  await expect(page.getByRole("button", { name: "Modifica", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Descrizione" })).toBeVisible();

  // Comments: tab, heading, composer placeholder/label, button and an Italian date.
  const tabs = page.getByTestId("activity-tabs");
  await expect(tabs.getByRole("button", { name: "Commenti" })).toBeVisible();
  await expect(tabs.getByRole("button", { name: "Cronologia" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Commenti" })).toBeVisible();
  await expect(page.getByText("Aggiungi un commento…")).toBeVisible();
  await expect(page.getByRole("textbox", { name: "Aggiungi un commento" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Aggiungi commento" })).toBeVisible();
  const commentCard = page.locator("div.rounded-lg").filter({ hasText: "Commento di prova" });
  await expect(commentCard).toBeVisible();
  await expect(commentCard).toContainText(IT_DATE);

  // Other sections.
  await expect(page.getByRole("heading", { name: "Allegati" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Monitoraggio del tempo" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Registra lavoro" })).toBeVisible();
  await expect(page.getByText("Ancora nessun sotto-task.")).toBeVisible();

  // History: Italian sentence with translated field name; values are data.
  await tabs.getByRole("button", { name: "Cronologia" }).click();
  const priorityRow = page.getByTestId("history-row").filter({ hasText: "priorità" });
  await expect(priorityRow).toHaveCount(1);
  await expect(priorityRow).toContainText("ha modificato");
  await expect(priorityRow).toContainText("«high»");
  await expect(priorityRow).toContainText("«low»");
  await expect(priorityRow).not.toContainText("changed");
  await expect(priorityRow).toContainText(/fa\b|ora/);
});

test("Crea ticket modal in Italian", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects");
  await page.getByRole("button", { name: "Crea", exact: true }).click();
  await page.getByRole("button", { name: "Ticket", exact: true }).click();
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: "Crea ticket" })).toBeVisible();
  await expect(modal.getByLabel("Progetto")).toBeVisible();
  await expect(modal.getByText("Seleziona un progetto…")).toBeAttached();
  await expect(modal.getByText("Riepilogo", { exact: true })).toBeVisible();
  await expect(modal.getByText("Priorità", { exact: true })).toBeVisible();
  await expect(modal.getByText("Assegnatario", { exact: true })).toBeVisible();
  await expect(modal.getByText("Creane un altro")).toBeVisible();
  await expect(modal.getByRole("button", { name: "Crea", exact: true })).toBeVisible();
  await expect(modal.getByRole("button", { name: "Annulla", exact: true })).toBeVisible();
  await expect(modal.getByRole("heading", { name: "Create issue" })).toHaveCount(0);
  await modal.getByRole("button", { name: "Annulla", exact: true }).click();
  await expect(modal).toHaveCount(0);
});
