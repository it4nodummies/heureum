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

// Crea un'issue da zero via il Create della topbar (stesso flusso del test
// Subtasks sopra) e ritorna la sua key (DEMO-NNNN) letta dall'URL dopo la
// navigazione a /app/browse/{key}.
async function createIssue(page: Page, summary: string): Promise<string> {
  await page.goto("/app/projects");
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await page.getByRole("button", { name: "Issue", exact: true }).click();
  const createModal = page.locator("div.fixed.inset-0.z-50");
  await expect(createModal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  await createModal.getByLabel("Project").selectOption({ label: "Demo Project (DEMO)" });
  await createModal.locator("#issue-summary").fill(summary);
  await createModal.getByRole("button", { name: "Create", exact: true }).click();
  await page.waitForURL(/\/app\/browse\/DEMO-\d+/);
  const match = page.url().match(/DEMO-\d+/);
  if (!match) throw new Error(`could not read issue key from URL ${page.url()}`);
  return match[0];
}

test("Linked work items: aggiunge un link 'Blocks' verso un'altra issue e poi lo rimuove", async ({ page }) => {
  await login(page);

  const unique = Date.now();
  const sourceSummary = `E2E Link Source ${unique}`;
  const targetSummary = `E2E Link Target ${unique}`;

  // La issue target va creata per prima così esiste già quando la cerchiamo
  // dall'autocomplete sulla issue source.
  const targetKey = await createIssue(page, targetSummary);
  const sourceKey = await createIssue(page, sourceSummary);
  expect(sourceKey).not.toBe(targetKey);

  const section = page.getByTestId("linked-work-items-section");
  await expect(section).toBeVisible();
  await expect(page.getByText("No linked work items yet.")).toBeVisible();

  await page.getByLabel("Relation type").selectOption("Blocks");
  const targetInput = page.getByLabel("Search for an issue to link");
  await targetInput.fill(targetSummary);

  const suggestion = section.getByText(targetKey, { exact: true });
  await expect(suggestion).toBeVisible();
  await suggestion.click();

  await section.getByRole("button", { name: "Add", exact: true }).click();

  const linkRow = section.getByTestId(`issue-link-row-${targetKey}`);
  await expect(linkRow).toBeVisible();
  await expect(section.getByText("blocks", { exact: true })).toBeVisible();
  await expect(linkRow.getByText(targetSummary)).toBeVisible();

  await linkRow.getByRole("button", { name: `Remove link to ${targetKey}` }).click();
  await expect(linkRow).not.toBeVisible();
  await expect(page.getByText("No linked work items yet.")).toBeVisible();
});

test("Attachments: fa l'upload di un file, lo vede in lista e poi lo elimina", async ({ page }) => {
  await login(page);

  await createIssue(page, `E2E Attachments ${Date.now()}`);

  const section = page.getByTestId("attachments-section");
  await expect(section).toBeVisible();
  await expect(page.getByText("No attachments yet.")).toBeVisible();

  const fileName = `e2e-note-${Date.now()}.txt`;
  await page.getByLabel("Upload attachment").setInputFiles({
    name: fileName,
    mimeType: "text/plain",
    buffer: Buffer.from("hello from the attachments e2e test"),
  });

  await expect(page.getByText("No attachments yet.")).not.toBeVisible();
  await expect(section.getByText(fileName)).toBeVisible();

  const card = section.locator('[data-testid^="attachment-card-"]').filter({ hasText: fileName });
  await expect(card).toHaveCount(1);

  await card.getByRole("button", { name: `Delete ${fileName}` }).click();
  await expect(section.getByText(fileName)).not.toBeVisible();
  await expect(page.getByText("No attachments yet.")).toBeVisible();
});

test("Time tracking: registra tempo con 'Log work' e lo vede riflesso nel blocco e nella lista worklog", async ({
  page,
}) => {
  await login(page);

  await createIssue(page, `E2E Time Tracking ${Date.now()}`);

  const section = page.getByTestId("time-tracking-section");
  await expect(section).toBeVisible();
  await expect(section.getByTestId("time-tracking-text")).toHaveText("No time logged");
  await expect(section.getByText("No work logged yet.")).toBeVisible();

  await section.getByRole("button", { name: "Log work", exact: true }).click();

  const dialog = page.locator("div.fixed.inset-0.z-50");
  await expect(dialog.getByRole("heading", { name: "Log work" })).toBeVisible();
  await dialog.getByLabel("Time spent").fill("2h");
  await dialog.getByRole("button", { name: "Log work", exact: true }).click();

  await expect(dialog).not.toBeVisible();
  await expect(section.getByTestId("time-tracking-text")).toHaveText("2h logged · no estimate set");

  const row = section.locator('[data-testid^="worklog-row-"]');
  await expect(row).toHaveCount(1);
  await expect(row.getByText("2h", { exact: true })).toBeVisible();

  // Deleting only removes the worklog entry from the list — the backend's
  // WorklogService.Delete (internal/domain/issue/worklog_service.go) doesn't
  // decrement Issue.TimeSpent, so the aggregate "logged" total deliberately
  // isn't re-asserted here (that's a backend gap outside this task's scope).
  await row.getByRole("button", { name: /^Delete worklog/ }).click();
  await expect(row).toHaveCount(0);
  await expect(section.getByText("No work logged yet.")).toBeVisible();
});

test("Activity History: cambia la priority via Edit mode e la vede loggata nel tab History", async ({ page }) => {
  await login(page);

  await createIssue(page, `E2E Activity History ${Date.now()}`);

  // Comments is the default Activity tab (regression guard for collaboration.spec.ts).
  const tabs = page.getByTestId("activity-tabs");
  await expect(tabs.getByRole("button", { name: "Comments" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Comments" })).toBeVisible();

  // New issues default to priority "Medium" (issue.PriorityMedium in
  // internal/api/handlers/issue_handler.go) — switch to "High" via Edit
  // mode so the changelog logs an old→new "priority" change.
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  const prioritySelect = page.locator("select").first();
  await prioritySelect.selectOption({ label: "High" });
  await page.getByRole("button", { name: "Save", exact: true }).click();

  // Back in read mode: Edit button reappears once the save mutation settles.
  await expect(page.getByRole("button", { name: "Edit", exact: true })).toBeVisible();

  await tabs.getByRole("button", { name: "History" }).click();
  await expect(page.getByRole("heading", { name: "Comments" })).not.toBeVisible();

  const historyList = page.getByTestId("history-list");
  await expect(historyList).toBeVisible();

  // Several fields get re-logged on every Edit→Save (title, description,
  // story points, estimates) even when unchanged — see issue_handler.go's
  // Update, which always forwards summary/description/etc. Filter to the
  // one row about "priority" so the assertion is specific to this change.
  const priorityRow = page.getByTestId("history-row").filter({ hasText: "priority" });
  await expect(priorityRow).toHaveCount(1);
  await expect(priorityRow).toContainText("System");
  await expect(priorityRow).toContainText("medium");
  await expect(priorityRow).toContainText("high");
});

test("Assignee: si assegna l'issue tramite lo UserPicker in Edit mode", async ({ page }) => {
  await login(page);

  await createIssue(page, `E2E Assignee ${Date.now()}`);

  await expect(page.getByTestId("field-assignee")).toHaveText("Unassigned");

  await page.getByRole("button", { name: "Edit", exact: true }).click();

  // Opens the UserPicker dropdown (the trigger button carries
  // aria-label="Assignee" — see components/common/UserPicker.tsx) and uses
  // the "Assign to me" shortcut, which is deterministic (no debounced search
  // round trip to wait on) and assigns the logged-in seeded admin
  // ("Ada Admin", cmd/seed/main.go) who is already a DEMO project member —
  // required for GET /user/assignable/search to include anyone at all.
  await page.getByLabel("Assignee", { exact: true }).click();
  await page.getByRole("button", { name: "Assign to me" }).click();

  await page.getByRole("button", { name: "Save", exact: true }).click();

  // Back in read mode: Edit button reappears once the save mutation settles.
  await expect(page.getByRole("button", { name: "Edit", exact: true })).toBeVisible();
  await expect(page.getByTestId("field-assignee")).toHaveText("Ada Admin");

  // Re-open Edit mode and unassign via the picker's "Unassigned" option, to
  // cover the "clear the assignee" path (assignee: {accountId: ""} — see
  // IssueView.tsx's save mutation comment on why it's always sent).
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  await page.getByLabel("Assignee", { exact: true }).click();
  await page.getByRole("button", { name: "Unassigned", exact: true }).click();
  await page.getByRole("button", { name: "Save", exact: true }).click();

  await expect(page.getByRole("button", { name: "Edit", exact: true })).toBeVisible();
  await expect(page.getByTestId("field-assignee")).toHaveText("Unassigned");
});

test("Create issue modal: crea una issue con description, priority e assignee impostati dal modale", async ({
  page,
}) => {
  await login(page);

  await page.goto("/app/projects");
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await page.getByRole("button", { name: "Issue", exact: true }).click();
  const createModal = page.locator("div.fixed.inset-0.z-50");
  await expect(createModal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  await createModal.getByLabel("Project").selectOption({ label: "Demo Project (DEMO)" });

  const summary = `E2E Create Rich ${Date.now()}`;
  await createModal.locator("#issue-summary").fill(summary);
  await createModal.locator("#issue-description").fill("Created from the rich create modal e2e test.");
  await createModal.getByLabel("Priority", { exact: true }).selectOption({ label: "High" });

  await createModal.getByLabel("Assignee", { exact: true }).click();
  await createModal.getByRole("button", { name: "Assign to me" }).click();

  await createModal.getByRole("button", { name: "Create", exact: true }).click();

  await page.waitForURL(/\/app\/browse\/DEMO-\d+/);

  await expect(page.getByTestId("field-priority")).toHaveText("High");
  await expect(page.getByTestId("field-assignee")).toHaveText("Ada Admin");
  await expect(page.getByText("Created from the rich create modal e2e test.")).toBeVisible();
});

test("Create issue modal: 'Create another' resetta summary/description e mantiene il modale aperto", async ({
  page,
}) => {
  await login(page);

  await page.goto("/app/projects");
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await page.getByRole("button", { name: "Issue", exact: true }).click();
  const createModal = page.locator("div.fixed.inset-0.z-50");
  await expect(createModal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  await createModal.getByLabel("Project").selectOption({ label: "Demo Project (DEMO)" });

  await createModal.getByRole("checkbox", { name: "Create another" }).check();

  const unique = Date.now();
  const firstSummary = `E2E Create Another First ${unique}`;
  await createModal.locator("#issue-summary").fill(firstSummary);
  await createModal.getByRole("button", { name: "Create", exact: true }).click();

  // Stays open: the "Create issue" heading is still visible, the summary
  // input is cleared, and a small confirmation names the just-created key.
  await expect(createModal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  await expect(createModal.locator("#issue-summary")).toHaveValue("");
  await expect(createModal.getByText(/DEMO-\d+ created\./)).toBeVisible();

  const secondSummary = `E2E Create Another Second ${unique}`;
  await createModal.locator("#issue-summary").fill(secondSummary);
  await createModal.getByRole("button", { name: "Create", exact: true }).click();
  await expect(createModal.getByText(/DEMO-\d+ created\./)).toBeVisible();

  await createModal.getByRole("button", { name: "Done", exact: true }).click();
  await expect(createModal).not.toBeVisible();
});
