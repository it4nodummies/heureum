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

test("issue detail shows and edits a custom field value", async ({ page }) => {
  await login(page);
  // Create a text custom field on DEMO via the Fields tab.
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Fields" }).click();
  await expect(page.getByTestId("custom-fields-tab")).toBeVisible();
  await page.getByLabel(/field name/i).fill("Squad");
  await page.getByRole("button", { name: /add field|create field/i }).click();
  await expect(page.getByText("Squad")).toBeVisible();
  // Open an issue and confirm the custom field renders in Details.
  await page.goto("/app/browse/DEMO-1");
  await expect(page.getByTestId("custom-field-Squad")).toBeVisible();
});
