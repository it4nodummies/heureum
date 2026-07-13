import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira\/projects/);
}

test("JQL search on filters page returns seeded issues", async ({ page }) => {
  await login(page);
  await page.goto("/jira/filters");
  await page.getByLabel("JQL").fill("project = DEMO");
  await page.getByRole("button", { name: "Search" }).click();

  // la list view mostra almeno una issue del progetto DEMO
  await expect(page.getByRole("cell", { name: /DEMO-1/ })).toBeVisible();
});

test("column toggle hides a column", async ({ page }) => {
  await login(page);
  await page.goto("/jira/filters");
  await page.getByLabel("JQL").fill("project = DEMO");
  await page.getByRole("button", { name: "Search" }).click();
  await expect(page.getByRole("columnheader", { name: "Priority" })).toBeVisible();

  // deseleziona la colonna Priority; scope al container "Columns" per non
  // collidere con l'intestazione di colonna "Priority" nella tabella.
  await page.getByLabel("Columns").getByLabel("Priority", { exact: true }).uncheck();
  await expect(page.getByRole("columnheader", { name: "Priority" })).toHaveCount(0);
});

test("save filter then run it from the sidebar", async ({ page }) => {
  await login(page);
  await page.goto("/jira/filters");
  await page.getByLabel("JQL").fill("project = DEMO ORDER BY created DESC");

  page.once("dialog", (d) => d.accept("E2E filter")); // prompt del nome
  await page.getByRole("button", { name: "Save filter" }).click();

  await expect(page.getByRole("button", { name: "E2E filter" })).toBeVisible();
  await page.getByRole("button", { name: "E2E filter" }).click();
  await expect(page.getByRole("cell", { name: /DEMO-1/ })).toBeVisible();
});
