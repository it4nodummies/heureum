import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app/);
}

test("workflow editor shows seeded statuses and adds a new one", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();

  // the seeded default statuses are visible
  await expect(page.getByTestId("workflow-statuses")).toBeVisible();
  await expect(page.getByTestId("status-TO DO")).toBeVisible();

  // add a new status
  await page.getByLabel("New status name").fill("Review");
  await page.getByLabel("Category (reporting only)").selectOption("inprogress");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByTestId("status-Review")).toBeVisible();
});

async function dragStatus(page: Page, fromTestId: string, toTestId: string) {
  const source = page.getByTestId(fromTestId);
  const target = page.getByTestId(toTestId);
  const sourceBox = await source.boundingBox();
  const targetBox = await target.boundingBox();
  if (!sourceBox || !targetBox) throw new Error("drag handle bounding box not found");
  await page.mouse.move(sourceBox.x + sourceBox.width / 2, sourceBox.y + sourceBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(targetBox.x + targetBox.width / 2, targetBox.y + targetBox.height / 2, { steps: 10 });
  await page.mouse.up();
}

test("workflow editor persists status order after drag-and-drop reorder", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();

  await expect(page.getByTestId("status-TO DO")).toBeVisible();
  await expect(page.getByTestId("status-DONE")).toBeVisible();

  const rowsBefore = await page.getByTestId("workflow-statuses").locator("li").allTextContents();
  expect(rowsBefore.findIndex((r) => r.includes("DONE"))).toBeGreaterThan(
    rowsBefore.findIndex((r) => r.includes("TO DO"))
  );

  // Drag "DONE" above "TO DO", and wait for the reorder PUT to actually resolve
  // before reloading — otherwise the reload can race ahead of the backend commit
  // and refetch the old order (the optimistic client-side update alone isn't proof
  // of persistence).
  const [response] = await Promise.all([
    page.waitForResponse(
      (res) => res.url().includes("/workflow/statuses/order") && res.request().method() === "PUT"
    ),
    dragStatus(page, "drag-handle-DONE", "drag-handle-TO DO"),
  ]);
  expect(response.ok()).toBeTruthy();

  await expect(async () => {
    const rows = await page.getByTestId("workflow-statuses").locator("li").allTextContents();
    expect(rows.findIndex((r) => r.includes("DONE"))).toBeLessThan(rows.findIndex((r) => r.includes("TO DO")));
  }).toPass();

  // Reload and verify the new order persisted server-side.
  await page.reload();
  await page.getByRole("button", { name: "Workflow" }).click();
  await expect(page.getByTestId("status-TO DO")).toBeVisible();
  await expect(page.getByTestId("status-DONE")).toBeVisible();
  const rowsAfter = await page.getByTestId("workflow-statuses").locator("li").allTextContents();
  expect(rowsAfter.findIndex((r) => r.includes("DONE"))).toBeLessThan(
    rowsAfter.findIndex((r) => r.includes("TO DO"))
  );
});

test("workflow editor creates and removes a transition between two statuses", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Workflow" }).click();

  // Two fresh statuses with no transitions between them yet.
  await page.getByLabel("New status name").fill("Test A");
  await page.getByLabel("Category (reporting only)").selectOption("inprogress");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByTestId("status-Test A")).toBeVisible();

  await page.getByLabel("New status name").fill("Test B");
  await page.getByLabel("Category (reporting only)").selectOption("inprogress");
  await page.getByRole("button", { name: "Add status" }).click();
  await expect(page.getByTestId("status-Test B")).toBeVisible();

  // Create a transition Test A -> Test B.
  await page.getByLabel("From status").selectOption({ label: "Test A" });
  await page.getByLabel("To status").selectOption({ label: "Test B" });
  await page.getByLabel("Transition name").fill("Test Transition");
  await page.getByRole("button", { name: "Add transition" }).click();
  await expect(page.getByTestId("transition-Test Transition")).toBeVisible();

  // Remove it again.
  await page.getByLabel("Delete transition Test Transition").click();
  await expect(page.getByTestId("transition-Test Transition")).not.toBeVisible();
});
