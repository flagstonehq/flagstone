import { test, expect } from "@playwright/test";
test.describe("Flags", () => {
  test("redirects unauthenticated users to /login", async ({ page }) => {
    await page.goto("/projects/my-app/flags");
    await expect(page).toHaveURL(/\/login/);
  });
});
