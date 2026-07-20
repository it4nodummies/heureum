import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app\/projects/);
}

// End-to-end admin flow for teams-associated-to-projects:
//  1. create a team on the Teams page (/app/groups) and add an existing user to it;
//  2. associate that team with the DEMO project as `member` via the Access tab
//     Teams section;
//  3. assert the association is visible.
//
// The "team member can now edit issues" flow relies on effective-role authz and
// is covered by the Go authz/contract tests; here we verify the admin-side
// association end-to-end with a visible confirmation.
test("create a team, add a member, and associate it with a project", async ({ page }) => {
  await login(page);

  // ── 1. Create a team and add an existing user ──────────────────────────────
  await page.goto("/app/groups");
  await expect(page.getByTestId("groups-admin")).toBeVisible();

  const teamName = `access-team-${Date.now()}`;
  await page.getByLabel(/team name/i).fill(teamName);
  await page.getByRole("button", { name: /create team/i }).click();

  // The new team appears in the "All teams" list.
  const teamRow = page.getByTestId("groups-list").getByText(teamName, { exact: true });
  await expect(teamRow).toBeVisible();

  // Expand the team and add the demo developer user.
  await teamRow.click();
  const addUserInput = page.getByLabel(new RegExp(`add user to ${teamName}`, "i"));
  await expect(addUserInput).toBeVisible();
  await addUserInput.fill("dev");
  await page.getByRole("button", { name: /Devi Developer/i }).click();
  await expect(
    page.getByTestId("group-member-row").filter({ hasText: /Devi Developer/i })
  ).toBeVisible();

  // ── 2. Associate the team with the DEMO project as `member` ────────────────
  await page.goto("/app/projects/DEMO/settings");
  await page.getByRole("button", { name: "Access" }).click();
  await expect(page.getByTestId("access-tab")).toBeVisible();

  // Pick the freshly created team in the "Add team" section and add it.
  const teamSelect = page.getByLabel("Team to add");
  await expect(teamSelect.locator("option", { hasText: teamName })).toBeAttached();
  await teamSelect.selectOption({ label: teamName });
  await page.getByLabel("New team role").selectOption("member");
  await page.getByRole("button", { name: /^Add$/ }).click();

  // ── 3. Assert the association shows in the Teams list ──────────────────────
  const associatedRow = page.getByTestId("team-row").filter({ hasText: teamName });
  await expect(associatedRow).toBeVisible();
  await expect(associatedRow.getByLabel(`Role for ${teamName}`)).toHaveValue("member");
});
