import { describe, it, expect } from "vitest";
import {
  loginSchema,
  createProjectSchema,
  createFlagSchema,
  changePasswordSchema,
  setupSchema,
  createApiKeySchema,
  createSegmentSchema,
  updateProjectSchema,
  createEnvironmentSchema,
} from "@/lib/schemas";
describe("loginSchema", () => {
  it("accepts valid input", () => {
    const result = loginSchema.safeParse({
      email: "admin@acme.com",
      password: "password123",
    });
    expect(result.success).toBe(true);
  });
  it("rejects invalid email", () => {
    const result = loginSchema.safeParse({
      email: "not-an-email",
      password: "password123",
    });
    expect(result.success).toBe(false);
  });
  it("rejects short password", () => {
    const result = loginSchema.safeParse({
      email: "admin@acme.com",
      password: "1234567",
    });
    expect(result.success).toBe(false);
  });
});
describe("createProjectSchema", () => {
  it("accepts valid slug", () => {
    const result = createProjectSchema.safeParse({
      name: "My App",
      slug: "my-app",
    });
    expect(result.success).toBe(true);
  });
  it("rejects slug with uppercase", () => {
    const result = createProjectSchema.safeParse({
      name: "My App",
      slug: "My-App",
    });
    expect(result.success).toBe(false);
  });
  it("rejects slug with spaces", () => {
    const result = createProjectSchema.safeParse({
      name: "My App",
      slug: "my app",
    });
    expect(result.success).toBe(false);
  });
});
describe("createFlagSchema", () => {
  it("accepts valid flag", () => {
    const result = createFlagSchema.safeParse({
      key: "new-checkout",
      name: "New Checkout Flow",
      type: "boolean",
    });
    expect(result.success).toBe(true);
  });
  it("rejects invalid key format", () => {
    const result = createFlagSchema.safeParse({
      key: "New Checkout",
      name: "New Checkout Flow",
      type: "boolean",
    });
    expect(result.success).toBe(false);
  });
  it("rejects invalid type", () => {
    const result = createFlagSchema.safeParse({
      key: "new-checkout",
      name: "New Checkout Flow",
      type: "invalid",
    });
    expect(result.success).toBe(false);
  });
});
describe("changePasswordSchema", () => {
  it("accepts matching passwords", () => {
    const result = changePasswordSchema.safeParse({
      current_password: "oldpass",
      new_password: "newpassword",
      confirm_password: "newpassword",
    });
    expect(result.success).toBe(true);
  });
  it("rejects non-matching passwords", () => {
    const result = changePasswordSchema.safeParse({
      current_password: "oldpass",
      new_password: "newpassword",
      confirm_password: "different",
    });
    expect(result.success).toBe(false);
  });
});
describe("setupSchema", () => {
  it("accepts valid setup", () => {
    const result = setupSchema.safeParse({
      tenant_name: "Acme Corp",
      tenant_slug: "acme",
      admin_email: "admin@acme.com",
      admin_password: "password123",
      confirm_password: "password123",
    });
    expect(result.success).toBe(true);
  });
  it("rejects non-matching passwords", () => {
    const result = setupSchema.safeParse({
      tenant_name: "Acme Corp",
      tenant_slug: "acme",
      admin_email: "admin@acme.com",
      admin_password: "password123",
      confirm_password: "different",
    });
    expect(result.success).toBe(false);
  });
});
describe("createApiKeySchema", () => {
  it("accepts valid key request", () => {
    const result = createApiKeySchema.safeParse({
      name: "Production Key",
      environment_id: "550e8400-e29b-41d4-a716-446655440000",
    });
    expect(result.success).toBe(true);
  });
  it("rejects invalid environment UUID", () => {
    const result = createApiKeySchema.safeParse({
      name: "Prod Key",
      environment_id: "not-a-uuid",
    });
    expect(result.success).toBe(false);
  });
});
describe("createSegmentSchema", () => {
  it("accepts valid segment", () => {
    const result = createSegmentSchema.safeParse({
      key: "beta-users",
      name: "Beta Users",
    });
    expect(result.success).toBe(true);
  });
  it("rejects key with spaces", () => {
    const result = createSegmentSchema.safeParse({
      key: "beta users",
      name: "Beta Users",
    });
    expect(result.success).toBe(false);
  });
});

describe("updateProjectSchema", () => {
  it("accepts valid name", () => {
    const r = updateProjectSchema.safeParse({ name: "My App" });
    expect(r.success).toBe(true);
  });
  it("rejects empty name", () => {
    const r = updateProjectSchema.safeParse({ name: "" });
    expect(r.success).toBe(false);
  });
});
describe("createEnvironmentSchema", () => {
  it("accepts valid slug", () => {
    const r = createEnvironmentSchema.safeParse({
      name: "Staging",
      slug: "staging",
    });
    expect(r.success).toBe(true);
  });
  it("rejects uppercase slug", () => {
    const r = createEnvironmentSchema.safeParse({
      name: "Staging",
      slug: "Staging",
    });
    expect(r.success).toBe(false);
  });
});
