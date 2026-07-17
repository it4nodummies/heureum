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

test("assign a fix version to an issue in Edit mode and persist it", async ({ page }) => {
  await login(page);

  // Create a release for DEMO so there is a version to assign.
  await page.goto("/app/projects/DEMO");
  await page.getByRole("link", { name: "Releases" }).click();
  await expect(page.getByTestId("releases-page")).toBeVisible();

  const name = `FixVer ${Date.now()}`;
  await page.getByLabel("Release name").fill(name);
  await page.getByRole("button", { name: "Create release" }).click();
  await expect(page.getByRole("row", { name: new RegExp(name) })).toBeVisible();

  // Open a seeded issue and enter Edit mode.
  await page.goto("/app/browse/DEMO-1");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await page.getByRole("button", { name: "Edit" }).click();

  // Assign the fix version via the multi-select, then save.
  await page.getByTestId("issue-fixversions-select").selectOption({ label: name });
  await page.getByRole("button", { name: "Save" }).click();

  // The view-mode row shows the assigned version.
  await expect(page.getByTestId("issue-fixversions")).toContainText(name);

  // Persisted across a reload.
  await page.reload();
  await expect(page.getByTestId("issue-fixversions")).toContainText(name);
});
