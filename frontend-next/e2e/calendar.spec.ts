import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("renders a month grid and navigates months", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO");
  await page.getByRole("link", { name: "Calendar" }).click();
  await expect(page).toHaveURL(/\/app\/projects\/DEMO\/calendar/);
  await expect(page.getByTestId("calendar-grid")).toBeVisible();
  // 28..31 day cells depending on month; assert at least 28 rendered.
  const cells = page.getByTestId("calendar-day");
  await expect(cells.nth(27)).toBeVisible();

  const title = page.getByTestId("calendar-title");
  const before = await title.textContent();
  await page.getByRole("button", { name: "Previous month" }).click();
  await expect(title).not.toHaveText(before ?? "");
});
