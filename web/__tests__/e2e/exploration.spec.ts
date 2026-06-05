import { test } from "@playwright/test";

const AUTH_FILE = "playwright/.auth/screenshots.json";

// ── Public pages ──────────────────────────────────────────────────────────────

test.describe("Public pages", () => {
  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
  });

  test("01 - Login page (empty)", async ({ page }) => {
    await page.goto("/login");
    await page.waitForLoadState("networkidle");
    await page.screenshot({ path: "screenshots/01-login-empty.png", fullPage: true });
  });

  test("02 - Login page (validation error)", async ({ page }) => {
    await page.goto("/login");
    await page.getByRole("button", { name: /sign in/i }).click();
    await page.waitForTimeout(500);
    await page.screenshot({ path: "screenshots/02-login-validation.png", fullPage: true });
  });

  test("03 - Login page (invalid credentials)", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/email/i).fill("wrong@example.com");
    await page.getByLabel(/password/i).fill("badpassword");
    await page.getByRole("button", { name: /sign in/i }).click();
    await page.waitForTimeout(1000);
    await page.screenshot({ path: "screenshots/03-login-error.png", fullPage: true });
  });

  test("14 - Setup page", async ({ page }) => {
    await page.goto("/setup");
    await page.waitForLoadState("networkidle");
    await page.screenshot({ path: "screenshots/14-setup-page.png", fullPage: true });
  });

  test("M01 - Login page (mobile 375px)", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/login");
    await page.waitForLoadState("networkidle");
    await page.screenshot({ path: "screenshots/M01-login-mobile.png", fullPage: true });
  });
});

// ── Authenticated pages (session loaded from auth.setup.ts) ───────────────────

test.describe("Authenticated pages", () => {
  test.use({ storageState: AUTH_FILE });

  test.beforeEach(async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
  });

  test("04 - Projects page", async ({ page }) => {
    await page.goto("/projects");
    await page.waitForLoadState("networkidle");
    await page.screenshot({ path: "screenshots/04-projects-list.png", fullPage: true });
  });

  test("05 - Flags page", async ({ page }) => {
    await page.goto("/projects/my-app/flags");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/05-flags-table.png", fullPage: true });
  });

  test("06 - Flags page (create dialog open)", async ({ page }) => {
    await page.goto("/projects/my-app/flags");
    await page.waitForLoadState("networkidle");
    await page.getByRole("button", { name: /new flag/i }).click();
    await page.waitForTimeout(500);
    await page.screenshot({ path: "screenshots/06-flags-create-dialog.png", fullPage: true });
  });

  test("07 - Rule editor (new-checkout, production)", async ({ page }) => {
    await page.goto("/projects/my-app/flags/new-checkout?env=production");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/07-rule-editor.png", fullPage: true });
  });

  test("08 - Rule editor (dark-mode, development)", async ({ page }) => {
    await page.goto("/projects/my-app/flags/dark-mode?env=development");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/08-rule-editor-dark-mode.png", fullPage: true });
  });

  test("09 - API Keys page", async ({ page }) => {
    await page.goto("/projects/my-app/api-keys");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/09-api-keys.png", fullPage: true });
  });

  test("10 - API Keys (create dialog)", async ({ page }) => {
    await page.goto("/projects/my-app/api-keys");
    await page.waitForLoadState("networkidle");
    await page.getByRole("button", { name: /new api key/i }).click();
    await page.waitForTimeout(500);
    await page.screenshot({ path: "screenshots/10-api-keys-create-dialog.png", fullPage: true });
  });

  test("11 - Audit log page", async ({ page }) => {
    await page.goto("/projects/my-app/audit");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/11-audit-log.png", fullPage: true });
  });

  test("12 - Settings page", async ({ page }) => {
    await page.goto("/projects/my-app/settings");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/12-settings.png", fullPage: true });
  });

  test("13 - Sidebar (flags page)", async ({ page }) => {
    await page.goto("/projects/my-app/flags");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(500);
    await page.screenshot({ path: "screenshots/13-sidebar-full.png", fullPage: true });
  });

  test("16 - Segments page", async ({ page }) => {
    await page.goto("/projects/my-app/segments");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/16-segments-page.png", fullPage: true });
  });

  // Mobile
  test("M02 - Flags page (mobile 375px)", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/projects/my-app/flags");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/M02-flags-mobile.png", fullPage: true });
  });

  test("M03 - Rule editor (mobile 375px)", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/projects/my-app/flags/new-checkout?env=production");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/M03-rule-editor-mobile.png", fullPage: true });
  });

  test("M04 - Settings page (mobile 375px)", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.goto("/projects/my-app/settings");
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(800);
    await page.screenshot({ path: "screenshots/M04-settings-mobile.png", fullPage: true });
  });
});
