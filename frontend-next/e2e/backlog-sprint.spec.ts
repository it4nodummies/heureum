import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("create a sprint with a name and goal, goal renders on the sprint header", async ({ page }) => {
  await login(page);
  await page.goto("/app/boards/1/backlog");
  await expect(page.getByLabel("New sprint name")).toBeVisible();

  const sprintName = `Goal Sprint ${Date.now()}`;
  const goal = `Ship the thing ${Date.now()}`;
  await page.getByLabel("New sprint name").fill(sprintName);
  await page.getByLabel("Sprint goal").fill(goal);
  await page.getByRole("button", { name: "Create sprint" }).click();

  await expect(page.getByText(sprintName)).toBeVisible();
  await expect(page.getByTestId("sprint-goal").filter({ hasText: goal })).toBeVisible();
});
