import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app/);
}

test("workflow editor shows seeded statuses and adds a new one", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();

  // the seeded default statuses are visible
  await expect(page.getByTestId("workflow-statuses")).toBeVisible();
  await expect(page.getByTestId("status-TO DO")).toBeVisible();

  // add a new status
  await page.getByLabel("New status name").fill("Review");
  await page.getByLabel("New status category").selectOption("inprogress");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByTestId("status-Review")).toBeVisible();
});
