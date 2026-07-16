import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("create a custom field and see it listed", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Fields" }).click();
  await expect(page.getByTestId("custom-fields-tab")).toBeVisible();
  await page.getByLabel(/field name/i).fill("Team name");
  await page.getByRole("button", { name: /add field|create field/i }).click();
  await expect(page.getByText("Team name")).toBeVisible();
});
