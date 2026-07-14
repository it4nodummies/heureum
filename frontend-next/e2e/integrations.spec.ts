import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira\/projects/);
}

test("integrations tab adds and lists a webhook", async ({ page }) => {
  await login(page);
  await page.goto("/jira/projects/DEMO/settings");
  await page.getByRole("button", { name: "Integrations" }).click();
  await expect(page.getByTestId("integrations-tab")).toBeVisible();

  await page.getByLabel("Webhook URL").fill("https://example.com/my-hook");
  await page.getByRole("button", { name: "Add webhook" }).click();
  await expect(page.getByText("https://example.com/my-hook")).toBeVisible();
});

test("issue development panel renders", async ({ page }) => {
  await login(page);
  await page.goto("/jira/browse/DEMO-1");
  await expect(page.getByTestId("development-panel")).toBeVisible();
});
