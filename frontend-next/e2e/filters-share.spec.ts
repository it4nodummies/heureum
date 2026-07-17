import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("saves a shared filter and shows a shared indicator", async ({ page }) => {
  await login(page);
  await page.goto("/app/filters");
  await page.getByLabel("JQL").fill("project = DEMO");
  await page.getByLabel(/filter name/i).fill("Shared DEMO");
  await page.getByLabel(/share with (the )?team/i).check();
  await page.getByRole("button", { name: /^save$/i }).click();

  await expect(page.getByRole("button", { name: "Shared DEMO" })).toBeVisible();
  await expect(page.getByTestId("filter-shared-badge").first()).toBeVisible();
});
