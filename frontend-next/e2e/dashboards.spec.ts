import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app/);
}

test("adds and removes a dashboard gadget", async ({ page }) => {
  await login(page);
  await page.goto("/app/dashboards");

  // Create a dashboard, then open its detail page.
  const dashName = `Gadget E2E ${Date.now()}`;
  await page.getByLabel("New dashboard name").fill(dashName);
  await page.getByRole("main").getByRole("button", { name: "Create" }).click();
  await page.getByRole("link", { name: dashName }).click();
  await page.waitForURL(/\/app\/dashboards\/.+/);

  // Add a gadget from the supported catalog.
  await page.getByLabel(/gadget type/i).selectOption("assigned_to_me");
  await page.getByRole("button", { name: /add gadget/i }).click();

  const gadget = page.getByTestId("gadget").filter({ hasText: /assigned to me/i });
  await expect(gadget.first()).toBeVisible();

  // Remove it.
  await gadget.first().getByRole("button", { name: /remove/i }).click();
  await expect(page.getByTestId("gadget").filter({ hasText: /assigned to me/i })).toHaveCount(0);
});
