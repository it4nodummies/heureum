import { test, expect, Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/jira\/projects/);
}

test("crea un nuovo progetto e lo vede nella lista", async ({ page }) => {
  await login(page);
  await page.getByRole("button", { name: /create project/i }).click();

  // Step 1: template picker. Scrum is selected by default, so we just advance.
  const modal = page.locator("div.fixed.inset-0.z-50");
  await expect(modal.getByRole("heading", { name: /select project type/i })).toBeVisible();
  await modal.getByRole("button", { name: "Next" }).click();

  // Step 2: name/key fields have no <label for>, so we target by placeholder
  // (see CreateProjectModal.tsx: "My awesome project" / "MAP" placeholders).
  await expect(modal.getByRole("heading", { name: "Create project" })).toBeVisible();
  await modal.locator('input[placeholder="My awesome project"]').fill("Marketing Site");
  await modal.locator('input[placeholder="MAP"]').fill("MKT");
  await modal.getByRole("button", { name: "Create project" }).click();

  await expect(page.getByText("Marketing Site")).toBeVisible();
});

test("apre le impostazioni del progetto DEMO dal menu azioni e rinomina", async ({ page }) => {
  await login(page);

  // Scope to the DEMO row (grid row carries both "group" and "cursor-pointer"
  // classes, unique to data rows vs. the header row) to avoid ambiguity with
  // any other project created by other tests.
  const demoRow = page.locator("div.group.cursor-pointer", { hasText: "Demo Project" });
  await demoRow.getByRole("button", { name: /project actions/i }).click();
  await demoRow.getByRole("button", { name: /project settings/i }).click();

  await page.waitForURL(/\/jira\/projects\/DEMO\/settings/);
  const nameInput = page.locator("#proj-name");
  await expect(nameInput).toHaveValue(/Demo Project/);
  await nameInput.fill("Demo Project Renamed");
  await page.getByRole("button", { name: /save changes/i }).click();
  await expect(page.getByText(/saved/i)).toBeVisible();
});
