import { test, expect, Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("mostra i commenti seedati e ne aggiunge uno", async ({ page }) => {
  await login(page);
  await page.goto("/app/browse/DEMO-1");

  await expect(page.getByRole("heading", { name: /comments/i })).toBeVisible();
  await expect(page.getByText("Grazie, ci sto lavorando.")).toBeVisible();
  await expect(page.getByText("Aggiornato lo stato.")).toBeVisible();

  await page.getByLabel(/add a comment/i).fill("Commento E2E");
  await page.getByRole("button", { name: /add comment/i }).click();

  await expect(page.getByText("Commento E2E")).toBeVisible();
});

test("watch e vote toggle", async ({ page }) => {
  await login(page);
  await page.goto("/app/browse/DEMO-2");

  // IssueView.tsx renders "Watch (N)" / "Stop watching (N)" and
  // "Vote (N)" / "Unvote (N)" as the accessible button names, so anchor the
  // regex at the start to avoid the watch button matching the vote name.
  const watchBtn = page.getByRole("button", { name: /^watch/i });
  const stopWatchingBtn = page.getByRole("button", { name: /^stop watching/i });
  const voteBtn = page.getByRole("button", { name: /^vote/i });
  const unvoteBtn = page.getByRole("button", { name: /^unvote/i });

  await expect(watchBtn.or(stopWatchingBtn)).toBeVisible();
  const wasWatching = await stopWatchingBtn.isVisible();
  if (wasWatching) {
    await stopWatchingBtn.click();
    await expect(watchBtn).toBeVisible();
  } else {
    await watchBtn.click();
    await expect(stopWatchingBtn).toBeVisible();
  }

  await expect(voteBtn.or(unvoteBtn)).toBeVisible();
  const hadVoted = await unvoteBtn.isVisible();
  if (hadVoted) {
    await unvoteBtn.click();
    await expect(voteBtn).toBeVisible();
  } else {
    await voteBtn.click();
    await expect(unvoteBtn).toBeVisible();
  }
});
