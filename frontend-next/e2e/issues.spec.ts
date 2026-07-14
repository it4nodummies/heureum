import { test, expect, Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("apre la vista di una issue seedata e mostra i campi", async ({ page }) => {
  await login(page);
  await page.goto("/app/browse/DEMO-1");

  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.getByText("Status", { exact: false })).toBeVisible();
  await expect(page.getByText("Priority", { exact: false })).toBeVisible();
});

test("modifica inline del summary di una issue", async ({ page }) => {
  await login(page);
  await page.goto("/app/browse/DEMO-2");

  const h1 = page.getByRole("heading", { level: 1 });
  await expect(h1).toBeVisible();
  await h1.click();

  // IssueView.tsx swaps the <h1> for a single <input> (border-[#0052cc],
  // text-2xl) while editing; it's the only <input> rendered on this page.
  const input = page.locator("input.text-2xl");
  await expect(input).toBeVisible();
  await input.fill("Summary modificato E2E");
  await input.press("Enter");

  await expect(page.getByRole("heading", { level: 1 })).toHaveText(/Summary modificato E2E/);
});
