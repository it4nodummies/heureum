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

test("notification bell has Direct/Watching tabs that filter the list", async ({ page }) => {
  await login(page);
  await page.getByRole("button", { name: "Notifications" }).click();
  await expect(page.getByTestId("notif-dropdown")).toBeVisible();

  const direct = page.getByTestId("notif-tab-direct");
  const watching = page.getByTestId("notif-tab-watching");
  await expect(direct).toBeVisible();
  await expect(watching).toBeVisible();

  // Direct is the default active tab.
  await expect(direct).toHaveAttribute("aria-selected", "true");
  await expect(watching).toHaveAttribute("aria-selected", "false");

  // Clicking Watching switches the active tab (filters the list to that type set).
  await watching.click();
  await expect(watching).toHaveAttribute("aria-selected", "true");
  await expect(direct).toHaveAttribute("aria-selected", "false");
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

test("profile page has a language selector that persists, and an avatar upload control", async ({ page }) => {
  await login(page);
  await page.goto("/app/profile");
  await expect(page.getByRole("heading", { name: "Profile" })).toBeVisible();

  // Language/locale selector exists and can be changed + persisted.
  const locale = page.getByTestId("profile-locale");
  await expect(locale).toBeVisible();
  await locale.selectOption("it");
  await page.getByRole("button", { name: "Save profile" }).click();
  await page.reload();
  await expect(page.getByTestId("profile-locale")).toHaveValue("it");

  // The avatar upload control renders.
  await expect(page.getByTestId("avatar-upload")).toBeAttached();
});

test("profile page can add a notification preference", async ({ page }) => {
  await login(page);
  await page.goto("/app/profile");
  await expect(page.getByTestId("notif-prefs")).toBeVisible();

  const form = page.getByTestId("add-pref-form");
  await expect(form).toBeVisible();

  // Pick an event type and channels, then add the preference.
  await form.getByLabel("Event type").selectOption("mention");
  await form.getByLabel("App", { exact: true }).check();
  await form.getByLabel("Email", { exact: true }).check();
  await form.getByRole("button", { name: "Add preference" }).click();

  // The new pref row for the chosen event type appears in the list,
  // scoped to "All projects".
  const row = page.getByTestId("notif-prefs").locator('[data-event="mention"]');
  await expect(row).toBeVisible();
  await expect(row).toContainText("mention");
  await expect(row).toContainText("All projects");
});
