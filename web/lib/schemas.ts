import { z } from "zod";
export const loginSchema = z.object({
  email: z.string().email("Must be a valid email"),
  password: z.string().min(8, "Password must be at least 8 characters"),
  tenant_slug: z.string().optional(),
});
export const createProjectSchema = z.object({
  name: z.string().min(1, "Name is required").max(100),
  slug: z
    .string()
    .min(1)
    .max(50)
    .regex(/^[a-z0-9-]+$/, "Only lowercase letters, numbers, and hyphens"),
});
export const createFlagSchema = z.object({
  key: z
    .string()
    .min(1)
    .max(100)
    .regex(/^[a-z0-9-_]+$/, "Lowercase letters, numbers, hyphens, underscores"),
  name: z.string().min(1, "Name is required").max(200),
  type: z.enum(["boolean", "string", "number", "json"]),
  description: z.string().max(500).optional(),
});
export const createApiKeySchema = z.object({
  name: z.string().min(1, "Name is required").max(100),
  environment_id: z.string().uuid("Must be a valid environment"),
  expires_at: z.string().datetime().optional(),
});
export const changePasswordSchema = z
  .object({
    current_password: z.string().min(1, "Current password is required"),
    new_password: z.string().min(8, "Password must be at least 8 characters"),
    confirm_password: z.string(),
  })
  .refine((d) => d.new_password === d.confirm_password, {
    message: "Passwords do not match",
    path: ["confirm_password"],
  });
export const setupSchema = z
  .object({
    tenant_name: z.string().min(1, "Name is required").max(100),
    tenant_slug: z
      .string()
      .min(1)
      .max(50)
      .regex(/^[a-z0-9-]+$/),
    admin_email: z.string().email("Must be a valid email"),
    admin_password: z.string().min(8, "Password must be at least 8 characters"),
    confirm_password: z.string(),
  })
  .refine((d) => d.admin_password === d.confirm_password, {
    message: "Passwords do not match",
    path: ["confirm_password"],
  });
export const createSegmentSchema = z.object({
  key: z
    .string()
    .min(1)
    .max(100)
    .regex(/^[a-z0-9-_]+$/),
  name: z.string().min(1, "Name is required").max(200),
  description: z.string().max(500).optional(),
});
