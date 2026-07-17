import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("shows the project timeline with bars", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO");
  await page.getByRole("link", { name: "Timeline" }).click();
  await expect(page).toHaveURL(/\/app\/projects\/DEMO\/timeline/);
  await expect(page.getByTestId("timeline-chart")).toBeVisible();
  // DEMO has at least one sprint (seed), so at least one bar renders.
  await expect(page.getByTestId("timeline-bar").first()).toBeVisible();
});
