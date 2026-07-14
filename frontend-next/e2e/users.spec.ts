import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app/);
}

test("notification bell opens dropdown", async ({ page }) => {
  await login(page);
  await page.getByRole("button", { name: "Notifications" }).click();
  await expect(page.getByTestId("notif-dropdown")).toBeVisible();
  await expect(page.getByTestId("notif-dropdown").getByText("Notifications")).toBeVisible();
});

test("profile page loads and saves display name", async ({ page }) => {
  await login(page);
  await page.goto("/app/profile");
  await expect(page.getByRole("heading", { name: "Profile" })).toBeVisible();
  await page.getByLabel("Display name").fill("Ada Lovelace");
  await page.getByRole("button", { name: "Save profile" }).click();
  await page.reload();
  await expect(page.getByLabel("Display name")).toHaveValue("Ada Lovelace");
});
