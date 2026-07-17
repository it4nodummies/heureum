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

test("a child whose parent is in the result set renders as an indented child-row", async ({
  page,
}) => {
  await login(page);
  await runDemoSearch(page);

  // The DEMO seed has a subtask on DEMO-1 (parent_id → DEMO-1). Since DEMO-1 is
  // also in the result set, its child must render as a child-row.
  const childRows = page.getByTestId("child-row");
  await expect(childRows.first()).toBeVisible();

  // The child row must still expose the standard row controls (selection + key
  // link), i.e. hierarchy is purely presentational and doesn't strip behaviour.
  const firstChild = childRows.first();
  await expect(firstChild.getByTestId("row-select")).toBeVisible();
  await expect(firstChild.getByRole("link")).toBeVisible();
});
