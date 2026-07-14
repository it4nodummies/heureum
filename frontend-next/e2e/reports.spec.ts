import { test, expect, type Page } from "@playwright/test";

async function login(page: Page) {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@example.com");
  await page.getByLabel(/password/i).fill("admin-demo-123");
  await page.locator('form button[type="submit"]').click();
  await page.waitForURL(/\/app/);
}

test("project reports page renders charts", async ({ page }) => {
  await login(page);
  await page.goto("/app/projects/DEMO/reports");
  await expect(page.getByRole("heading", { name: /Reports/i })).toBeVisible();
  // almeno il grafico velocity o CFD o la torta rende un SVG con testid
  await expect(page.getByTestId("pie-chart")).toBeVisible();
  // cambia il campo della torta
  await page.getByLabel("Pie field").selectOption("priority");
  await expect(page.getByTestId("pie-chart")).toBeVisible();
});

test("dashboards page lists and creates a dashboard", async ({ page }) => {
  await login(page);
  await page.goto("/app/dashboards");
  await expect(page.getByRole("heading", { name: /Dashboards/i })).toBeVisible();
  await page.getByLabel("New dashboard name").fill("E2E Dashboard");
  await page.getByRole("main").getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("link", { name: "E2E Dashboard" })).toBeVisible();
});
