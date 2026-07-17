import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("create a release, toggle released, and filter", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO");

  // Navigate to the Releases tab.
  await page.getByRole("link", { name: "Releases" }).click();
  await expect(page.getByTestId("releases-page")).toBeVisible();

  // Create a release with a unique name + a release date.
  const name = `Release ${Date.now()}`;
  await page.getByLabel("Release name").fill(name);
  await page.getByLabel("Release date").fill("2026-12-31");
  await page.getByRole("button", { name: "Create release" }).click();

  // It appears in the table as Unreleased.
  const row = page.getByRole("row", { name: new RegExp(name) });
  await expect(row).toBeVisible();
  await expect(row.getByText("Unreleased")).toBeVisible();

  // Toggle it released; the status shows Released.
  await row.getByRole("button", { name: /release/i }).click();
  await expect(row.getByText("Released", { exact: true })).toBeVisible();

  // Filter to Released only — the released version stays visible.
  await page.getByLabel("Status filter").selectOption("released");
  await expect(page.getByRole("row", { name: new RegExp(name) })).toBeVisible();

  // Filter to Unreleased only — the released version is hidden.
  await page.getByLabel("Status filter").selectOption("unreleased");
  await expect(page.getByRole("row", { name: new RegExp(name) })).toHaveCount(0);
});
