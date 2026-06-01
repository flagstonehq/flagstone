import { test, expect } from "@playwright/test";

test.describe("Navigation", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/email/i).fill("admin@acme.com");
    await page.getByLabel(/password/i).fill("password123");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/projects/, { timeout: 5000 });
  });

  test("sidebar shows global nav on projects page", async ({ page }) => {
    await expect(page.getByRole("link", { name: /projects/i })).toBeVisible();
  });

  test("sidebar shows project nav items when viewing a project", async ({ page }) => {
    await page.goto("/projects/my-app/flags");

    // All sidebar links should be visible
    await expect(page.getByRole("link", { name: /flags/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /segments/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /api keys/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /audit/i })).toBeVisible();
    await expect(page.getByRole("link", { name: /settings/i })).toBeVisible();
  });

  test("navigates between pages via sidebar", async ({ page }) => {
    await page.goto("/projects/my-app/flags");

    // Click Settings in sidebar
    await page.getByRole("link", { name: /settings/i }).click();
    await expect(page).toHaveURL(/\/projects\/my-app\/settings/);

    // Click back to Flags
    await page.getByRole("link", { name: /flags/i }).click();
    await expect(page).toHaveURL(/\/projects\/my-app\/flags/);
  });

  test("all projects link navigates back to projects list", async ({ page }) => {
    await page.goto("/projects/my-app/flags");

    await page.getByRole("link", { name: /all projects/i }).click();
    await expect(page).toHaveURL(/\/projects$/);
  });
});
