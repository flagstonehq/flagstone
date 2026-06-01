import { test, expect } from "@playwright/test";

test.describe("Rule Editor", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/email/i).fill("admin@acme.com");
    await page.getByLabel(/password/i).fill("password123");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/projects/, { timeout: 5000 });
  });

  test("loads the rule editor for a flag", async ({ page }) => {
    await page.goto("/projects/my-app/flags/new-checkout?env=production");

    await expect(page).toHaveURL(/\/flags\/new-checkout/);
    await expect(page.getByRole("heading", { name: /new checkout/i })).toBeVisible();
    await expect(page.getByText(/environment/i)).toBeVisible();
    await expect(page.getByText(/enabled/i)).toBeVisible();
  });

  test("shows the save button", async ({ page }) => {
    await page.goto("/projects/my-app/flags/new-checkout?env=production");

    await expect(page.getByRole("button", { name: /save/i })).toBeVisible();
  });
});
