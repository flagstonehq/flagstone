import { test, expect } from "@playwright/test";

test.describe("Flags", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/email/i).fill("admin@acme.com");
    await page.getByLabel(/password/i).fill("password123");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/projects/, { timeout: 5000 });
  });

  test("shows flag rows for my-app project", async ({ page }) => {
    await page.goto("/projects/my-app/flags");
    await expect(page).toHaveURL(/\/projects\/my-app\/flags/);

    await expect(page.getByText("new-checkout")).toBeVisible();
    await expect(page.getByText("dark-mode")).toBeVisible();
  });

  test("shows environment columns in flags table", async ({ page }) => {
    await page.goto("/projects/my-app/flags");
    await expect(page.getByText(/development/i).first()).toBeVisible();
    await expect(page.getByText(/production/i).first()).toBeVisible();
  });

  test("opens create flag dialog", async ({ page }) => {
    await page.goto("/projects/my-app/flags");

    await page.getByRole("button", { name: /new flag/i }).click();

    await expect(page.getByRole("heading", { name: "Create flag" })).toBeVisible();
    await expect(page.getByLabel(/key/i)).toBeVisible();
  });

  test("navigates to rule editor when clicking a flag key", async ({ page }) => {
    await page.goto("/projects/my-app/flags");

    await page.getByRole("link", { name: "new-checkout" }).click();

    await expect(page).toHaveURL(/\/flags\/new-checkout/);
    await expect(page.getByRole("heading", { name: /new checkout/i })).toBeVisible();
  });

  test("create flag dialog shows validation errors on empty submit", async ({ page }) => {
    await page.goto("/projects/my-app/flags");

    await page.getByRole("button", { name: /new flag/i }).click();
    await expect(page.getByRole("heading", { name: "Create flag" })).toBeVisible();

    await page.getByRole("button", { name: /create flag/i }).click();

    await expect(page.getByText(/required/i)).toBeVisible();
  });
});
