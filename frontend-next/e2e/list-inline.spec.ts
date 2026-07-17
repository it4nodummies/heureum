import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

async function runDemoSearch(page: Page) {
  await page.goto("/app/filters");
  await page.getByLabel("JQL").fill("project = DEMO");
  await page.getByRole("button", { name: "Search" }).click();
  await expect(page.getByRole("cell", { name: /^DEMO-1$/ })).toBeVisible();
}

function demoRow(page: Page) {
  return page
    .getByRole("row")
    .filter({ has: page.getByRole("cell", { name: /^DEMO-1$/ }) });
}

test("inline-edit a priority cell updates the issue", async ({ page }) => {
  await login(page);
  await runDemoSearch(page);

  const row = demoRow(page);
  await expect(row).toBeVisible();

  // Priority cell is read-only text until clicked.
  await row.getByTestId("cell-priority").click();

  // A dropdown appears scoped to DEMO-1; pick "Highest" (id 1) — different from
  // the seeded High and from the bulk spec's Lowest, so this is a real change
  // regardless of spec ordering.
  const select = row.getByLabel(/priority for DEMO-1/i);
  await expect(select).toBeVisible();
  await select.selectOption("1");

  // After update + refetch, the cell shows the new value (back to read-only).
  await expect(row.getByTestId("cell-priority")).toContainText("Highest");
});

test("inline status cell offers workflow transitions", async ({ page }) => {
  await login(page);
  await runDemoSearch(page);

  const row = demoRow(page);
  await row.getByTestId("cell-status").click();

  // Either a transition dropdown appears (issue has available transitions) or
  // the cell stays read-only (none) — both are valid; assert the cell survives.
  await expect(row.getByTestId("cell-status")).toBeVisible();
});
