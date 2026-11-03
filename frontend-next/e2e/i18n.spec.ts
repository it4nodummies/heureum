import { test, expect, type APIRequestContext, type Page } from "@playwright/test";

// Round 29 — i18n core: locale resolution (saved profile locale → heureum_locale
// cookie → browser language → en), no-flash server <html lang>, the Italian
// chrome (TopBar/Sidebar), login, profile and a localized notification.
//
// The suite shares ONE database (workers: 1): every user whose locale this spec
// changes is restored to "" (unset) in afterAll.

const API = process.env.E2E_API_URL ?? "http://localhost:8080";

type Creds = { email: string; password: string };
const ADMIN: Creds = { email: "admin@example.com", password: "admin-demo-123" };
const DEV: Creds = { email: "dev@example.com", password: "dev-demo-123" };

async function authHeaders(request: APIRequestContext, who: Creds) {
  const res = await request.post(`${API}/rest/api/3/auth/login`, { data: who });
  expect(res.status(), `login ${who.email}`).toBe(200);
  const { token } = (await res.json()) as { token: string };
  return { Authorization: `Bearer ${token}` };
}

async function setSavedLocale(request: APIRequestContext, who: Creds, locale: string) {
  const res = await request.put(`${API}/rest/api/3/myself`, {
    headers: await authHeaders(request, who),
    data: { locale },
  });
  expect(res.status(), `PUT /myself locale=${locale} for ${who.email}`).toBe(200);
}

async function login(page: Page, who: Creds) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill(who.email);
  await page.getByLabel(/password/i).fill(who.password);
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

function collectHydrationErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on("console", (msg) => {
    if ((msg.type() === "error" || msg.type() === "warning") && /hydrat/i.test(msg.text())) {
      errors.push(msg.text());
    }
  });
  page.on("pageerror", (err) => {
    if (/hydrat/i.test(err.message)) errors.push(err.message);
  });
  return errors;
}

async function expectItalianNav(page: Page) {
  const sidebar = page.locator("aside");
  await expect(sidebar.getByText("Progetti", { exact: true })).toBeVisible();
  await expect(sidebar.getByText("Filtri", { exact: true })).toBeVisible();
  await expect(sidebar.getByText("Dashboard", { exact: true })).toBeVisible();
  await expect(sidebar.getByText("Projects", { exact: true })).toHaveCount(0);
  await expect(page.locator("html")).toHaveAttribute("lang", "it");
}

async function expectEnglishNav(page: Page) {
  const sidebar = page.locator("aside");
  await expect(sidebar.getByText("Projects", { exact: true })).toBeVisible();
  await expect(sidebar.getByText("Filters", { exact: true })).toBeVisible();
  await expect(sidebar.getByText("Dashboards", { exact: true })).toBeVisible();
  await expect(sidebar.getByText("Progetti", { exact: true })).toHaveCount(0);
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
}

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  await setSavedLocale(request, ADMIN, "");
  await setSavedLocale(request, DEV, "");
  await request.dispose();
});

test.afterAll(async ({ playwright }) => {
  const request = await playwright.request.newContext();
  await setSavedLocale(request, ADMIN, "");
  await setSavedLocale(request, DEV, "");
  await request.dispose();
});

test("(a) choosing Italiano in the profile translates the chrome at once and on reload, without hydration warnings", async ({ page }) => {
  const hydration = collectHydrationErrors(page);
  await login(page, ADMIN);
  await expectEnglishNav(page);

  await page.goto("/app/profile");
  await page.getByTestId("profile-locale").selectOption("it");
  await page.getByRole("button", { name: "Save profile" }).click();

  // Applied immediately, no reload.
  await expectItalianNav(page);
  await expect(page.getByRole("heading", { name: "Profilo" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Crea", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Salva profilo" })).toBeVisible();

  // The server renders <html lang="it"> from the cookie (no flash of English).
  const html = await (await page.request.get("/app/profile")).text();
  expect(html).toMatch(/<html[^>]*\blang="it"/);

  await page.reload();
  expect(await page.evaluate(() => document.documentElement.lang)).toBe("it");
  await expect(page.getByRole("heading", { name: "Profilo" })).toBeVisible();
  await expect(page.getByTestId("profile-locale")).toHaveValue("it");
  await expectItalianNav(page);
  expect(hydration).toEqual([]);
});

test("(a2) after an Italian user signs out, a user with an empty locale follows the (en-US) browser, not the cookie", async ({ page, request }) => {
  await setSavedLocale(request, ADMIN, "it");
  await setSavedLocale(request, DEV, "");
  await login(page, ADMIN);
  await expectItalianNav(page);

  // Client-side sign-out (no reload): the heureum_locale cookie still says "it".
  await page.getByTestId("user-menu-button").click();
  await page.getByRole("button", { name: "Esci" }).click();
  await page.waitForURL(/\/login/);
  await page.getByLabel(/email/i).fill(DEV.email);
  await page.getByLabel(/password/i).fill(DEV.password);
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
  await expectEnglishNav(page);
});

test.describe("Italian browser (it-IT)", () => {
  test.use({ locale: "it-IT" });

  test("(b) empty profile locale follows the browser: Italian login and app", async ({ page }) => {
    const hydration = collectHydrationErrors(page);
    await page.goto("/login");
    expect(await page.evaluate(() => document.documentElement.lang)).toBe("it");
    await expect(page.locator('form button[type="submit"]')).toHaveText("Accedi");
    await expect(page.getByRole("button", { name: "Crea account" })).toBeVisible();

    await login(page, DEV);
    await expectItalianNav(page);
    expect(hydration).toEqual([]);
  });

  test("(c) a saved English locale wins over the Italian browser", async ({ page }) => {
    // Admin still has "it" saved from (a).
    await login(page, ADMIN);
    await expectItalianNav(page);

    await page.goto("/app/profile");
    await page.getByTestId("profile-locale").selectOption("en");
    await page.getByRole("button", { name: "Salva profilo" }).click();
    await expectEnglishNav(page);
    await expect(page.getByRole("heading", { name: "Profile" })).toBeVisible();

    await page.reload();
    await expectEnglishNav(page);
  });

  test("(d) a legacy locale (es) counts as unset: Italian UI, profile shows Default", async ({ page, request }) => {
    await setSavedLocale(request, ADMIN, "es");
    await login(page, ADMIN);
    await expectItalianNav(page);

    await page.goto("/app/profile");
    await expect(page.getByTestId("profile-locale")).toHaveValue("");
    await expect(page.getByTestId("profile-locale").locator("option")).toHaveText([
      "Predefinita (browser)",
      "English",
      "Italiano",
    ]);
  });

  test("(e) notifications are rendered in Italian from their type", async ({ page, request }) => {
    const headers = await authHeaders(request, ADMIN);
    const users = await request.get(`${API}/rest/api/3/user/search?query=dev`, { headers });
    expect(users.status()).toBe(200);
    const dev = ((await users.json()) as { accountId: string; emailAddress?: string }[]).find(
      (u) => u.emailAddress === DEV.email
    );
    expect(dev, "dev user").toBeTruthy();

    const summary = `Notifica i18n ${Date.now()}`;
    const created = await request.post(`${API}/rest/api/3/issue`, {
      headers,
      data: {
        fields: {
          project: { key: "DEMO" },
          summary,
          issuetype: { name: "Task" },
          assignee: { accountId: dev!.accountId },
        },
      },
    });
    expect(created.status(), await created.text()).toBe(201);
    const { key } = (await created.json()) as { key: string };

    await login(page, DEV);
    await page.getByRole("button", { name: "Notifiche" }).click();
    const dropdown = page.getByTestId("notif-dropdown");
    await expect(dropdown.getByText("Segna tutte come lette")).toBeVisible();
    await expect(page.getByTestId("notif-tab-direct")).toContainText("Dirette");

    const item = dropdown.locator("li", { hasText: summary });
    await expect(item).toContainText("Ti è stato assegnato un ticket");
    await expect(item).toContainText(`Ti è stato assegnato ${key}: ${summary}`);
    await expect(item.locator("time")).toHaveText(/(\bfa|^ora)$/);
  });
});

// ── Sweep: no half-Italian screen ─────────────────────────────────────────────
// An Italian user (saved locale "it", en-US browser) visits every main route on
// fixtures from a dedicated project whose names/summaries are Italian, and no
// visible text, placeholder, aria-label or title on the page (top bar, sidebar,
// <main>, open menus and modals) may contain one of these English chrome words
// (whole word, case-sensitive).
// Data that collides (other specs' names in shared lists) is excluded by
// selector, never by weakening the list.

const SENTINELS = [
  "Create", "Save", "Cancel", "Delete", "Settings", "Search", "Assignee", "Reporter", "Priority",
  "Status", "Summary", "Description", "Comment", "Loading", "No results", "Issue", "Issues", "Add",
  "Edit", "Remove", "Close", "Back", "Filter", "Filters", "Projects", "Members", "Release date",
  "Released", "Unassigned", "Sprint goal",
];
const SENTINEL_RES = SENTINELS.map(
  (w) => [w, new RegExp(`(?<![\\p{L}\\p{N}_])${w.replace(/ /g, "\\s+")}(?![\\p{L}\\p{N}_])`, "u")] as const,
);

// Italian short date, e.g. "3 nov 2026" (Intl, same options as useFormat().date).
const IT_DATE = /(?<!\d)\d{1,2} (gen|feb|mar|apr|mag|giu|lug|ago|set|ott|nov|dic) \d{4}(?!\d)/;

/** Visible text nodes + placeholder/aria-label/title of visible elements on the page. */
async function chromeTexts(page: Page, ignore: string[]): Promise<string[]> {
  return page.evaluate((ignore) => {
    const out: string[] = [];
    const skipped = (el: Element) => ignore.some((sel) => el.closest(sel));
    const visible = (el: Element) =>
      typeof (el as HTMLElement).checkVisibility === "function"
        ? (el as HTMLElement).checkVisibility({ visibilityProperty: true } as CheckVisibilityOptions)
        : true;
    for (const root of [document.body]) {
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
      for (let n = walker.nextNode(); n; n = walker.nextNode()) {
        const el = n.parentElement;
        const text = (n.textContent ?? "").trim();
        if (!el || !text || skipped(el) || el.closest("script, style")) continue;
        const holder = el.tagName === "OPTION" ? el.closest("select") : el;
        if (!holder || !visible(holder)) continue;
        out.push(text);
      }
      for (const el of Array.from(root.querySelectorAll("[placeholder], [aria-label], [title]"))) {
        if (skipped(el) || !visible(el)) continue;
        for (const attr of ["placeholder", "aria-label", "title"]) {
          const v = el.getAttribute(attr);
          if (v) out.push(`${attr}="${v}"`);
        }
      }
    }
    return out;
  }, ignore);
}

// Data the backend scaffolds in English (not chrome): removed from a text
// before matching, only where it occurs.
const SCAFFOLDED_DATA = ["Back to In Progress"];

async function expectNoEnglish(page: Page, where: string, ignore: string[] = []) {
  await expect(page.locator("html"), `${where}: html[lang]`).toHaveAttribute("lang", "it");
  await expect(page.getByText(/^Caricamento/)).toHaveCount(0);
  const leftovers: string[] = [];
  for (const raw of await chromeTexts(page, ignore)) {
    const text = SCAFFOLDED_DATA.reduce((acc, d) => acc.split(d).join(""), raw);
    for (const [word, re] of SENTINEL_RES) if (re.test(text)) leftovers.push(`${word} ← ${JSON.stringify(raw)}`);
  }
  expect.soft(leftovers, `${where}: English chrome words`).toEqual([]);
}

test.describe("Italian UI sweep", () => {
  const stamp = Date.now().toString().slice(-6);
  const scrumKey = `GI${stamp}`;
  const kanbanKey = `GK${stamp}`;
  const scrumName = `Giro italiano ${stamp}`;
  const filterName = `Filtro del giro ${stamp}`;
  const dashboardName = `Cruscotto del giro ${stamp}`;
  const teamName = `squadra-giro-${stamp}`;
  const jql = `project = ${scrumKey} ORDER BY created ASC`;
  let scrumBoard = 0;
  let kanbanBoard = 0;
  let storyKey = "";
  let dashboardId = "";

  async function boardOf(request: APIRequestContext, headers: Record<string, string>, key: string) {
    for (let startAt = 0; ; startAt += 50) {
      const res = await request.get(`${API}/rest/agile/1.0/board?startAt=${startAt}&maxResults=50`, { headers });
      expect(res.status(), "list boards").toBe(200);
      const { values, isLast } = (await res.json()) as {
        values: { id: number; location?: { projectKey?: string } }[];
        isLast?: boolean;
      };
      const board = values.find((b) => b.location?.projectKey === key);
      if (board) return board.id;
      if (isLast !== false || values.length === 0) throw new Error(`no board for ${key}`);
    }
  }

  test.beforeAll(async ({ playwright }) => {
    const request = await playwright.request.newContext();
    const headers = await authHeaders(request, ADMIN);
    const post = async (path: string, data: unknown, what: string) => {
      const res = await request.post(`${API}${path}`, { headers, data });
      expect(res.status(), `${what}: ${await res.text()}`).toBeLessThan(300);
      return res.status() === 204 ? {} : ((await res.json()) as Record<string, unknown>);
    };
    const put = async (path: string, data: unknown, what: string) => {
      const res = await request.put(`${API}${path}`, { headers, data });
      expect(res.status(), `${what}: ${await res.text()}`).toBeLessThan(300);
    };
    const issue = async (project: string, summary: string, type: string, extra: Record<string, unknown> = {}) =>
      (await post(
        "/rest/api/3/issue",
        { fields: { project: { key: project }, summary, issuetype: { name: type }, ...extra } },
        `create ${summary}`,
      )).key as string;

    for (const [key, name, kind] of [
      [scrumKey, scrumName, "scrum"],
      [kanbanKey, `Flusso continuo ${stamp}`, "kanban"],
    ] as const) {
      await post(
        "/rest/api/3/project",
        { key, name, projectTypeKey: "software", projectTemplateKey: `com.pyxis.greenhopper.jira:gh-${kind}-template` },
        `create project ${key}`,
      );
    }
    scrumBoard = await boardOf(request, headers, scrumKey);
    kanbanBoard = await boardOf(request, headers, kanbanKey);

    const release = await post(
      "/rest/api/3/version",
      { project: scrumKey, name: `rilascio-${stamp}`, releaseDate: "2026-12-31" },
      "create version",
    );
    const epicKey = await issue(scrumKey, "Pagamenti online", "Epic");
    storyKey = await issue(scrumKey, "Carrello degli acquisti", "Story", { parent: { key: epicKey } });
    await put(
      `/rest/api/3/issue/${storyKey}`,
      { fields: { customfield_10016: 3, fixVersions: [{ id: String(release.id) }] } },
      "story points + release",
    );
    await issue(scrumKey, "Calcolo del totale", "Subtask", { parent: { key: storyKey } });
    const taskKey = await issue(scrumKey, "Revisione del codice", "Task");
    await post(
      `/rest/api/3/issue/${storyKey}/comment`,
      { body: { type: "doc", version: 1, content: [{ type: "paragraph", content: [{ type: "text", text: "Va bene così" }] }] } },
      "comment",
    );
    const sprint = await post(
      "/rest/agile/1.0/sprint",
      { name: `Iterazione ${stamp}`, originBoardId: scrumBoard, goal: "Chiudere il carrello" },
      "create sprint",
    );
    await post(`/rest/agile/1.0/sprint/${sprint.id}/issue`, { issues: [storyKey, taskKey] }, "move into sprint");
    await post(`/rest/agile/1.0/sprint/${sprint.id}`, { state: "active" }, "start sprint");
    await post(
      "/rest/agile/1.0/sprint",
      { name: `Iterazione futura ${stamp}`, originBoardId: scrumBoard, startDate: "2027-01-04", endDate: "2027-01-18" },
      "create future sprint",
    );
    await issue(kanbanKey, "Aggiornare la documentazione", "Task");
    // Assigned to the admin: an "Assegnati a me" gadget row and a notification.
    const me = await request.get(`${API}/rest/api/3/myself`, { headers });
    const { accountId } = (await me.json()) as { accountId: string };
    await issue(scrumKey, "Riunione settimanale", "Task", { assignee: { accountId } });

    await post("/rest/api/3/filter", { name: filterName, jql }, "create filter");
    const dash = await post("/rest/api/3/dashboards", { name: dashboardName }, "create dashboard");
    dashboardId = String(dash.id);
    for (const widget_type of ["assigned_to_me", "activity_stream"]) {
      await post(`/rest/api/3/dashboards/${dashboardId}/widgets`, { widget_type, config_json: "{}" }, `add ${widget_type}`);
    }
    await post("/rest/api/3/group", { name: teamName }, "create team");

    await setSavedLocale(request, ADMIN, "it");
    await request.dispose();
  });

  test.afterAll(async ({ playwright }) => {
    const request = await playwright.request.newContext();
    await setSavedLocale(request, ADMIN, "");
    await request.dispose();
  });

  test("every main page is fully Italian", async ({ page }) => {
    test.setTimeout(180_000);
    await login(page, ADMIN);

    // Home redirects to the projects list (filtered to the fixture: other
    // specs' project names are data).
    await page.goto("/app");
    await page.waitForURL(/\/app\/projects$/);
    await expect(page.getByRole("heading", { name: "Progetti" })).toBeVisible();
    await page.getByPlaceholder("Cerca progetti…").fill(stamp);
    // The search is debounced: wait until only the two fixture projects are listed.
    await expect(page.getByText("2 progetti", { exact: true })).toBeVisible();
    await expect(page.getByText(scrumName)).toBeVisible();
    await expectNoEnglish(page, "projects list");

    // Menus and modals reachable from the top bar and the projects list.
    await page.getByRole("button", { name: "Crea", exact: true }).click();
    await expectNoEnglish(page, "top bar › create menu");
    await page.locator("header").getByRole("button", { name: "Ticket", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Crea ticket" })).toBeVisible();
    await page.waitForLoadState("networkidle");
    // The project picker lists every project of the shared database (names are data).
    await expectNoEnglish(page, "create issue modal", ['#issue-project option:not([value=""])']);
    await page.getByRole("button", { name: "Annulla", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Crea ticket" })).toHaveCount(0);

    await page.locator("main").getByRole("button", { name: "Crea progetto" }).first().click();
    await expectNoEnglish(page, "create project modal");
    await page.getByRole("button", { name: "Chiudi", exact: true }).click();

    await page.getByTestId("user-menu-button").click();
    await expectNoEnglish(page, "top bar › user menu");
    await page.getByTestId("user-menu-button").click();

    await page.getByRole("button", { name: "Notifiche" }).click();
    await expect(page.getByTestId("notif-dropdown")).toBeVisible();
    await expectNoEnglish(page, "top bar › notifications");
    await page.getByRole("button", { name: "Notifiche" }).click();

    await page.getByRole("button", { name: "Azioni del progetto" }).first().click();
    await expectNoEnglish(page, "projects list › project actions");
    await page.keyboard.press("Escape");

    await page.goto(`/app/projects/${scrumKey}`);
    await expect(page.getByText("Carrello degli acquisti").first()).toBeVisible();
    await expectNoEnglish(page, "project overview");

    await page.goto(`/app/projects/${scrumKey}/settings`);
    for (const tab of ["Generale", "Workflow", "Riepilogo", "Integrazioni", "Automazioni", "Campi personalizzati", "Accesso"]) {
      await page.getByRole("button", { name: tab, exact: true }).click();
      await page.waitForLoadState("networkidle");
      await expectNoEnglish(page, `project settings › ${tab}`);
    }

    for (const [path, anchor] of [
      ["releases", page.getByText(`rilascio-${stamp}`)],
      ["reports", page.getByText("Burndown")],
      ["calendar", page.getByTestId("calendar-grid")],
      ["timeline", page.getByText(`Iterazione ${stamp}`)],
    ] as const) {
      await page.goto(`/app/projects/${scrumKey}/${path}`);
      await expect(anchor.first()).toBeVisible();
      await page.waitForLoadState("networkidle");
      await expectNoEnglish(page, `project ${path}`);
    }

    for (const [board, label] of [
      [scrumBoard, "scrum"],
      [kanbanBoard, "kanban"],
    ] as const) {
      const sample = label === "scrum" ? "Carrello degli acquisti" : "Aggiornare la documentazione";
      await page.goto(`/app/boards/${board}`);
      await expect(page.getByText(sample).first()).toBeVisible();
      await page.waitForLoadState("networkidle");
      await expectNoEnglish(page, `${label} board`);

      await page.goto(`/app/boards/${board}/backlog`);
      await expect(page.getByText(label === "scrum" ? "Revisione del codice" : sample).first()).toBeVisible();
      await page.waitForLoadState("networkidle");
      await expectNoEnglish(page, `${label} backlog`);
      if (label === "scrum") {
        await page.getByRole("button", { name: "Modifica sprint" }).first().click();
        await expectNoEnglish(page, "scrum backlog › edit sprint");
        await page.getByRole("button", { name: "Annulla", exact: true }).first().click();
        await page.getByRole("button", { name: "Completa sprint" }).first().click();
        await expect(page.getByRole("dialog")).toBeVisible();
        await expectNoEnglish(page, "scrum backlog › complete sprint");
        await page.getByRole("button", { name: "Annulla", exact: true }).first().click();
      }

      await page.goto(`/app/boards/${board}/settings`);
      await expect(page.getByTestId("board-settings")).toBeVisible();
      await page.waitForLoadState("networkidle");
      await expectNoEnglish(page, `${label} board settings`);
    }

    await page.goto(`/app/browse/${storyKey}`);
    await expect(page.getByRole("heading", { name: "Carrello degli acquisti" })).toBeVisible();
    await page.waitForLoadState("networkidle");
    await expect(page.locator("main").getByText(IT_DATE).first()).toBeVisible();
    await expectNoEnglish(page, "issue detail › comments");
    await page.getByTestId("activity-tabs").getByRole("button", { name: "Cronologia" }).click();
    await expect(page.getByTestId("history-list")).toBeVisible();
    await expectNoEnglish(page, "issue detail › history");

    await page.goto(`/app/filters?jql=${encodeURIComponent(jql)}`);
    await expect(page.getByTestId("list-count")).toBeVisible();
    await page.waitForLoadState("networkidle");
    await expectNoEnglish(page, "filters + search results");

    await page.goto("/app/dashboards");
    await expect(page.getByText(dashboardName)).toBeVisible();
    await expectNoEnglish(page, "dashboards list");

    await page.goto(`/app/dashboards/${dashboardId}`);
    await expect(page.getByTestId("gadget")).toHaveCount(2);
    await page.waitForLoadState("networkidle");
    await expectNoEnglish(page, "dashboard detail");

    await page.goto("/app/groups");
    await expect(page.getByTestId("groups-list").getByText(teamName, { exact: true })).toBeVisible();
    await page.getByTestId("groups-list").getByText(teamName, { exact: true }).click();
    await page.waitForLoadState("networkidle");
    await expectNoEnglish(page, "teams");

    await page.goto("/app/profile");
    await expect(page.getByRole("heading", { name: "Profilo" })).toBeVisible();
    await page.waitForLoadState("networkidle");
    await expectNoEnglish(page, "profile");
  });
});
