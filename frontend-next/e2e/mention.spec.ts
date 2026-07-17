import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("comment editor: @mention autocomplete inserts a mention chip", async ({ page }) => {
  await login(page);
  await page.goto("/app/browse/DEMO-1");

  const commentEditor = page.getByTestId("comment-editor");
  await commentEditor.scrollIntoViewIfNeeded();
  await commentEditor.click();

  // Type "@" + a query token to open the autocomplete over project members.
  await commentEditor.pressSequentially("@Dev");

  const dropdown = page.getByTestId("mention-autocomplete");
  await expect(dropdown).toBeVisible();

  // Pick "Devi Developer" from the results.
  await dropdown.getByText("Devi Developer").click();
  await expect(dropdown).not.toBeVisible();

  await page.getByRole("button", { name: "Add comment" }).click();

  // The rendered comment shows the mention chip (@Devi Developer).
  const chip = page.locator("span[data-mention]", { hasText: "@Devi Developer" });
  await expect(chip).toBeVisible();
});
