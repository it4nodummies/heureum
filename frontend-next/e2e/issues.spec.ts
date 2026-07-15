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

test("modifica del summary di una issue via Edit mode", async ({ page }) => {
  await login(page);
  await page.goto("/app/browse/DEMO-2");

  const h1 = page.getByRole("heading", { level: 1 });
  await expect(h1).toBeVisible();
  await page.getByRole("button", { name: "Edit" }).click();

  // IssueView.tsx swaps the <h1> for a single <input> (border-[#0052cc],
  // text-2xl) while editing; it's the only <input> rendered on this page.
  const input = page.locator("input.text-2xl");
  await expect(input).toBeVisible();
  await input.fill("Summary modificato E2E");
  await page.getByRole("button", { name: "Save" }).click();

  await expect(page.getByRole("heading", { level: 1 })).toHaveText(/Summary modificato E2E/);
});

test("Edit mode: aggiunge una descrizione e cambia la priority", async ({ page }) => {
  await login(page);

  // Una issue creata da zero (CreateIssueModal non imposta una description)
  // non ha descrizione -> verifichiamo il placeholder e poi la aggiungiamo
  // via Edit mode.
  await page.goto("/app/projects");
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await page.getByRole("button", { name: "Issue", exact: true }).click();
  const createModal = page.locator("div.fixed.inset-0.z-50");
  await expect(createModal.getByRole("heading", { name: "Create issue" })).toBeVisible();
  await createModal.getByLabel("Project").selectOption({ label: "Demo Project (DEMO)" });
  await createModal.locator("#issue-summary").fill("E2E Edit Mode Issue");
  await createModal.getByRole("button", { name: "Create", exact: true }).click();

  await page.waitForURL(/\/app\/browse\/DEMO-\d+/);

  // Nessuna descrizione impostata alla creazione -> placeholder in read mode.
  await expect(page.getByText("No description")).toBeVisible();

  await page.getByRole("button", { name: "Edit" }).click();

  const textarea = page.getByPlaceholder("Add a description…");
  await expect(textarea).toBeVisible();
  await textarea.fill("Descrizione aggiunta via E2E test.");

  const prioritySelect = page.locator("select").first();
  await prioritySelect.selectOption({ label: "High" });

  await page.getByRole("button", { name: "Save" }).click();

  // Torna in read mode: la textarea di edit sparisce e il testo compare nel renderer ADF.
  await expect(page.getByPlaceholder("Add a description…")).not.toBeVisible();
  await expect(page.getByText("Descrizione aggiunta via E2E test.")).toBeVisible();
  await expect(page.getByText("High", { exact: false })).toBeVisible();
});
