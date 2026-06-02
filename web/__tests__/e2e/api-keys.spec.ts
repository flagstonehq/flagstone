import { test, expect } from "@playwright/test";

test.describe("API Keys", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/email/i).fill("admin@acme.com");
    await page.getByLabel(/password/i).fill("password123");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/projects/, { timeout: 5000 });
  });

  test("shows the API keys page", async ({ page }) => {
    await page.goto("/projects/my-app/api-keys");
    await expect(page.getByRole("heading", { name: /api keys/i })).toBeVisible();
    await expect(page.getByRole("button", { name: /new api key/i })).toBeVisible();
  });

  test("opens create dialog with environment selector", async ({ page }) => {
    await page.goto("/projects/my-app/api-keys");
    await page.getByRole("button", { name: /new api key/i }).click();

    await expect(page.getByRole("heading", { name: /new api key/i })).toBeVisible();
    await expect(page.getByLabel(/name/i)).toBeVisible();
    await expect(page.getByRole("combobox")).toBeVisible();
  });

  test("shows validation error when name is empty", async ({ page }) => {
    await page.goto("/projects/my-app/api-keys");
    await page.getByRole("button", { name: /new api key/i }).click();
    await expect(page.getByRole("heading", { name: /new api key/i })).toBeVisible();

    await page.getByRole("button", { name: /^create/i }).click();
    await expect(page.getByText(/required/i)).toBeVisible();
  });

  test("create key shows raw key modal once then hides it", async ({ page }) => {
    await page.goto("/projects/my-app/api-keys");
    await page.getByRole("button", { name: /new api key/i }).click();
    await expect(page.getByRole("heading", { name: /new api key/i })).toBeVisible();

    await page.getByLabel(/name/i).fill("e2e-test-key");

    // Select the first available environment
    await page.getByRole("combobox").click();
    await page.getByRole("option").first().click();

    await page.getByRole("button", { name: /^create/i }).click();

    // Raw key modal must appear
    await expect(page.getByText(/this key will not be shown again/i)).toBeVisible({ timeout: 5000 });

    const rawKeyInput = page.getByRole("textbox").filter({ hasText: /^fs_/ }).or(
      page.locator("input[readonly]"),
    );
    await expect(rawKeyInput.first()).toBeVisible();

    // Close the modal
    await page.getByRole("button", { name: /done|close|dismiss/i }).first().click();

    // Raw key must not be visible after closing
    await expect(page.getByText(/this key will not be shown again/i)).not.toBeVisible();
  });

  test("revoke button opens confirm dialog", async ({ page }) => {
    await page.goto("/projects/my-app/api-keys");

    const revokeBtn = page.getByRole("button", { name: /revoke/i }).first();
    const hasKeys = await revokeBtn.isVisible().catch(() => false);

    if (!hasKeys) {
      test.skip();
      return;
    }

    await revokeBtn.click();
    await expect(page.getByText(/are you sure|confirm/i)).toBeVisible();
  });
});
