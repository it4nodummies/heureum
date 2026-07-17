import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("select-all reveals the bulk bar and bulk-sets priority", async ({ page }) => {
  await login(page);
  await page.goto("/app/filters");
  await page.getByLabel("JQL").fill("project = DEMO");
  await page.getByRole("button", { name: "Search" }).click();

  // wait for the result table to render
  await expect(page.getByRole("cell", { name: /^DEMO-1$/ })).toBeVisible();

  // no bulk bar until something is selected
  await expect(page.getByTestId("bulk-bar")).toHaveCount(0);

  // select-all across the currently shown rows
  await page.getByTestId("select-all").check();

  const bar = page.getByTestId("bulk-bar");
  await expect(bar).toBeVisible();
  await expect(bar).toContainText(/selected/i);

  // pick "Lowest" (id 5) — no seeded DEMO issue starts as Lowest
  await bar.getByLabel("Bulk priority").selectOption("5");
  await bar.getByRole("button", { name: /^apply$/i }).click();

  // after the bulk update + refetch, a priority cell now shows Lowest
  await expect(page.getByRole("cell", { name: "Lowest" }).first()).toBeVisible();
  // selection cleared → bulk bar gone
  await expect(page.getByTestId("bulk-bar")).toHaveCount(0);
});
