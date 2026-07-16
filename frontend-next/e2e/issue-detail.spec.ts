import { test, expect, Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

test("Subtasks: crea un'issue, aggiunge un sottotask inline e vede il contatore aggiornarsi", async ({ page }) => {
  await login(page);

  // Crea una issue "genitore" da zero via il Create della topbar (stesso
  // flusso di issues.spec.ts) così il test non dipende dai dati seedati.
  await page.goto("/app/projects");
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await page.getByRole("button", { name: "Issue", exact: true }).click();
  const createModal = page.locator("div.fixed.inset-0.z-50");
  await expect(createModal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  await createModal.getByLabel("Project").selectOption({ label: "Demo Project (DEMO)" });
  await createModal.locator("#issue-summary").fill("E2E Subtasks Parent Issue");
  await createModal.getByRole("button", { name: "Create", exact: true }).click();

  await page.waitForURL(/\/app\/browse\/DEMO-\d+/);

  const subtasksSection = page.getByTestId("subtasks-section");
  await expect(subtasksSection).toBeVisible();
  await expect(page.getByText("No subtasks yet.")).toBeVisible();
  await expect(page.getByTestId("subtasks-progress")).toHaveText("0 of 0 done");

  const subtaskInput = page.getByLabel("Add a subtask");
  await subtaskInput.fill("A subtask added inline");
  await subtaskInput.press("Enter");

  await expect(page.getByText("A subtask added inline")).toBeVisible();
  await expect(page.getByTestId("subtasks-progress")).toHaveText("0 of 1 done");
  await expect(page.getByText("No subtasks yet.")).not.toBeVisible();

  // Il sottotask creato riceve davvero il tipo "Subtask" (regressione Round 13
  // Task 1/2: senza risoluzione per nome, tutte le issue create dalla UI
  // finivano mostrate come "Task" — vedi TypeIDByName in issue/service.go).
  const row = subtasksSection.locator('[data-testid^="subtask-row-"]');
  await expect(row).toHaveCount(1);
  await expect(row.getByTitle("Subtask")).toBeVisible();
});
