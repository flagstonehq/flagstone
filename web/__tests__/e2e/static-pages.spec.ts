import { test, expect } from "@playwright/test";

test.describe("Static Pages", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/email/i).fill("admin@acme.com");
    await page.getByLabel(/password/i).fill("password123");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/projects/, { timeout: 5000 });
  });

  test("API keys page loads", async ({ page }) => {
    await page.goto("/projects/my-app/api-keys");
    await page.waitForLoadState("networkidle");
    await expect(page.getByRole("heading", { name: /api keys/i })).toBeVisible();
  });

  test("Audit page loads", async ({ page }) => {
    await page.goto("/projects/my-app/audit");
    await page.waitForLoadState("networkidle");
    await expect(page.getByRole("heading", { name: /audit/i })).toBeVisible();
  });

  test("Settings page loads", async ({ page }) => {
    await page.goto("/projects/my-app/settings");
    await page.waitForLoadState("networkidle");
    await expect(page.getByRole("heading", { name: /settings/i })).toBeVisible();
  });

  test("Segments page returns 404", async ({ page }) => {
    const response = await page.goto("/projects/my-app/segments");
    expect(response?.status()).toBe(404);
  });
});
