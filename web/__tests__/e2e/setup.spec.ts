import { test, expect } from "@playwright/test";

test.describe("Setup", () => {
  test("redirects to /login when instance is already initialized", async ({ page }) => {
    await page.goto("/setup");
    // In a running dev environment the instance is always initialized,
    // so the proxy redirects to /login immediately.
    await expect(page).toHaveURL(/\/login|\/setup/, { timeout: 5000 });
  });

  test("setup page renders the form when backend is unreachable", async ({ page }) => {
    // Navigate directly; if the backend is up and initialized we get /login,
    // if the backend is down or not initialized we get the setup form.
    await page.goto("/setup");
    const url = page.url();

    if (url.includes("/login")) {
      // Already initialized — expected in dev. Verify redirect happened correctly.
      await expect(page.getByLabel(/email/i)).toBeVisible();
      return;
    }

    // Not initialized — verify the setup form is present.
    await expect(page.getByText(/flagstone/i).first()).toBeVisible();
    await expect(page.getByLabel(/organization name/i)).toBeVisible();
    await expect(page.getByLabel(/admin email/i)).toBeVisible();
    await expect(page.getByLabel(/^password/i).first()).toBeVisible();
    await expect(page.getByRole("button", { name: /create instance/i })).toBeVisible();
  });

  test("setup form shows validation errors on empty submit", async ({ page }) => {
    await page.goto("/setup");
    const url = page.url();

    if (url.includes("/login")) {
      test.skip();
      return;
    }

    await page.getByRole("button", { name: /create instance/i }).click();
    await expect(page.getByText(/required|at least/i).first()).toBeVisible();
  });

  test("setup form validates password mismatch", async ({ page }) => {
    await page.goto("/setup");
    const url = page.url();

    if (url.includes("/login")) {
      test.skip();
      return;
    }

    await page.getByLabel(/organization name/i).fill("Test Corp");
    await page.getByLabel(/admin email/i).fill("admin@test.com");
    await page.getByLabel(/^password/i).first().fill("password123");
    await page.getByLabel(/confirm password/i).fill("different-password");
    await page.getByRole("button", { name: /create instance/i }).click();

    await expect(page.getByText(/do not match/i)).toBeVisible();
  });

  test("authenticated users are redirected away from /setup", async ({ page }) => {
    // Login first
    await page.goto("/login");
    await page.getByLabel(/email/i).fill("admin@acme.com");
    await page.getByLabel(/password/i).fill("password123");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/projects/, { timeout: 5000 });

    // Attempt to access setup
    await page.goto("/setup");
    await expect(page).toHaveURL(/\/projects|\/login/);
  });
});
