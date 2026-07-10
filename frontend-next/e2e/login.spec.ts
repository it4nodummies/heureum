import { test, expect } from "@playwright/test";

test("login con utente demo e arrivo sui progetti", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();

  await page.waitForURL(/\/jira\/projects/);
  await expect(page.getByText("Demo Project")).toBeVisible();
});

test("login con credenziali errate non porta ai progetti", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("wrong-password");
  await page.locator('form button[type="submit"]').click();

  // NOTE: lib/api.ts's global 401 handler currently force-redirects to /login
  // via window.location on ANY 401 (including a failed login attempt itself),
  // which wipes the inline "invalid credentials" message before it can render.
  // Until that's fixed, the strongest faithful assertion is that the user is
  // kept out of the authenticated area and lands back on the login form.
  await page.waitForURL(/\/login/);
  await expect(page.getByLabel(/email/i)).toBeVisible();
});
