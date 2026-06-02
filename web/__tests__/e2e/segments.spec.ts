import { test, expect } from "@playwright/test";

test.describe("Segments", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/email/i).fill("admin@acme.com");
    await page.getByLabel(/password/i).fill("password123");
    await page.getByRole("button", { name: /sign in/i }).click();
    await expect(page).toHaveURL(/\/projects/, { timeout: 5000 });
  });

  test("shows segments list for my-app project", async ({ page }) => {
    await page.goto("/projects/my-app/segments");
    await expect(page).toHaveURL(/\/projects\/my-app\/segments/);

    await expect(page.getByText("beta-users")).toBeVisible();
    await expect(page.getByText("Beta Users")).toBeVisible();
    await expect(page.getByText("premium-customers")).toBeVisible();
    await expect(page.getByText("Premium Customers")).toBeVisible();
  });

  test("opens create segment dialog", async ({ page }) => {
    await page.goto("/projects/my-app/segments");

    await page.getByRole("button", { name: /new segment/i }).click();

    await expect(page.getByRole("heading", { name: /create segment/i })).toBeVisible();
    await expect(page.getByLabel(/key/i)).toBeVisible();
    await expect(page.getByLabel(/name/i)).toBeVisible();
  });

  test("navigates to segment detail page when clicking a segment key", async ({ page }) => {
    await page.goto("/projects/my-app/segments");

    await page.getByRole("link", { name: "beta-users" }).click();

    await expect(page).toHaveURL(/\/segments\/beta-users/);
    await expect(page.getByRole("heading", { name: /beta users/i })).toBeVisible();
  });

  test("shows rule editor on segment detail page", async ({ page }) => {
    await page.goto("/projects/my-app/segments/beta-users");

    await expect(page.getByText(/conditions for/i)).toBeVisible();
    await expect(page.getByText("Beta Users")).toBeVisible();
  });

  test("create segment dialog shows validation errors on empty submit", async ({ page }) => {
    await page.goto("/projects/my-app/segments");

    await page.getByRole("button", { name: /new segment/i }).click();
    await expect(page.getByRole("heading", { name: /create segment/i })).toBeVisible();

    await page.getByRole("button", { name: /create segment/i }).click();

    await expect(page.getByText(/required/i)).toBeVisible();
  });

  test("segment detail page shows archived section for archived segment", async ({ page }) => {
    await page.goto("/projects/my-app/segments/internal-tools");
    await expect(page.getByText(/this segment has been archived/i)).toBeVisible();
  });
});
