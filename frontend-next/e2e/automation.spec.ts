import { test, expect, type Page } from "@playwright/test";

// inline login (no e2e/helpers.ts in repo) — copied from export.spec.ts
async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("create and list an automation rule", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Automation" }).click();
  await expect(page.getByTestId("automation-tab")).toBeVisible();
  await page.getByRole("button", { name: /new rule|create rule/i }).click();
  await page.getByLabel(/rule name/i).fill("Auto-assign on create");
  // trigger select defaults to issue_created; submit
  await page.getByTestId("automation-tab").getByRole("button", { name: /^create$|save rule/i }).click();
  await expect(page.getByText("Auto-assign on create")).toBeVisible();
});
