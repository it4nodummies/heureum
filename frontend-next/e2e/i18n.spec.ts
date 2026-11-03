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
