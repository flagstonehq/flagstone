import { test, expect } from "@playwright/test";

test.describe("Projects", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/email/i).fill("admin@acme.com");
    await page.getByLabel(/password/i).fill("password123");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/projects/, { timeout: 5000 });
  });

  test("shows the my-app project card", async ({ page }) => {
    await expect(page.getByText("my-app")).toBeVisible();
    await expect(page.getByText("my-app", { exact: false }).first()).toBeVisible();
  });

  test("navigates to flags when clicking a project", async ({ page }) => {
    await page.getByRole("link", { name: /my-app/ }).first().click();
    await expect(page).toHaveURL(/\/projects\/my-app\/flags/);
  });
});
