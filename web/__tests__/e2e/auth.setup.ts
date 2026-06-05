import { test as setup } from "@playwright/test";
import path from "path";

export const AUTH_FILE = path.join(
  __dirname,
  "../../playwright/.auth/screenshots.json"
);

setup("save auth state", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel(/email/i).fill("admin@acme.com");
  await page.getByLabel(/password/i).fill("password123");
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(/\/projects/, { timeout: 10000 });
  await page.context().storageState({ path: AUTH_FILE });
});
