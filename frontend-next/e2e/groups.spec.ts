import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("create a team and see it listed", async ({ page }) => {
  await login(page);
  await page.goto("/app/groups");
  await expect(page.getByTestId("groups-admin")).toBeVisible();

  const name = `qa-team-${Date.now()}`;
  await page.getByLabel(/team name/i).fill(name);
  await page.getByRole("button", { name: /create team/i }).click();
  await expect(page.getByText(name, { exact: true })).toBeVisible();
});

test("Teams link in the sidebar navigates to the teams page", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects");
  await page.getByRole("link", { name: "Teams" }).click();
  await page.waitForURL(/\/app\/groups/);
  await expect(page.getByTestId("groups-admin")).toBeVisible();
});
