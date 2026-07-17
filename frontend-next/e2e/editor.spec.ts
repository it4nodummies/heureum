import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("description editor: applica Bold, salva e ricarica → rende <strong>", async ({ page }) => {
  await login(page);
  await page.goto("/app/browse/DEMO-1");

  await page.getByRole("button", { name: "Edit" }).click();

  const editor = page.getByTestId("description-editor");
  await expect(editor).toBeVisible();

  // Svuota l'eventuale descrizione esistente, digita testo, selezionalo, Bold.
  await editor.click();
  await page.keyboard.press("ControlOrMeta+a");
  await page.keyboard.press("Backspace");
  await editor.pressSequentially("BoldWordE2E");
  await editor.click({ clickCount: 3 }); // seleziona la riga
  // Scoping: anche il comment editor ha una toolbar con "Bold"; prendiamo
  // quella del RichTextEditor della descrizione (contenitore comune).
  const descRoot = editor.locator("xpath=../..");
  await descRoot.getByRole("button", { name: "Bold" }).click();

  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("description-editor")).not.toBeVisible();

  await page.reload();
  await expect(page.locator("strong", { hasText: "BoldWordE2E" })).toBeVisible();
});

test("comment editor: bullet list → rende <ul><li>", async ({ page }) => {
  await login(page);
  await page.goto("/app/browse/DEMO-1");

  const commentEditor = page.getByTestId("comment-editor");
  await commentEditor.scrollIntoViewIfNeeded();
  await commentEditor.click();
  await commentEditor.pressSequentially("BulletItemE2E");
  await page.getByRole("button", { name: "Bullet list" }).click();

  await page.getByRole("button", { name: "Add comment" }).click();

  const item = page.locator("ul li", { hasText: "BulletItemE2E" });
  await expect(item).toBeVisible();
});
