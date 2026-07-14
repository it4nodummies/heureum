import { test, expect } from "@playwright/test";

test("login con utente demo e arrivo sui progetti", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();

  await page.waitForURL(/\/app\/projects/);
  await expect(page.getByText("Demo Project")).toBeVisible();
});

test("login con credenziali errate mostra errore e resta su /login", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("wrong-password");
  await page.locator('form button[type="submit"]').click();

  // lib/api.ts's apiFetch now skips the global 401-redirect for the login
  // request itself (skipAuthRedirect), so the backend's "invalid credentials"
  // error (auth_handler.go: {"error":"invalid credentials"}, HTTP 401) propagates
  // to the login page's catch block and renders inline instead of bouncing the
  // user back to an empty /login form.
  await expect(page.getByText(/invalid|credenziali|incorrect/i)).toBeVisible();
  await expect(page).toHaveURL(/\/login/);
});
