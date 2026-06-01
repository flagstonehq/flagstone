export type Tenant = {
  id: string;
  slug: string;
  name: string;
  role: "owner" | "admin" | "member" | "viewer";
};
export type Project = {
  id: string;
  tenantId: string;
  slug: string;
  name: string;
  createdAt: string;
  updatedAt: string;
};
export type Environment = {
  id: string;
  projectId: string;
  slug: string;
  name: string;
};
export type Flag = {
  id: string;
  projectId: string;
  key: string;
  name: string;
  description: string | null;
  type: "boolean" | "string" | "number" | "json";
  defaultValue: unknown;
  archivedAt: string | null;
  createdAt: string;
  updatedAt: string;
  flagEnvironments?: FlagEnvironment[];
};
export type FlagEnvironment = {
  flagId: string;
  environmentId: string;
  enabled: boolean;
  rules: Rule[];
  version: number;
};
export type Rule = {
  id: string;
  conditions: Condition[];
  returnValue: boolean | string | number;
  rollout: number;
};
export type Condition = {
  attribute: string;
  // Must match exactly the operator strings the Go engine accepts.
  // Unknown operators evaluate silently to false — typos here break flags
  // without any error in the dashboard or the API.
  operator:
    | "eq"
    | "neq"
    | "gt"
    | "gte"
    | "lt"
    | "lte"
    | "in"
    | "not_in"
    | "contains"
    | "starts_with"
    | "ends_with"
    | "matches"
    | "exists"
    | "not_exists"
    | "segment";
  value: string | number | boolean | string[];
};
export type Segment = {
  id: string;
  projectId: string;
  key: string;
  name: string;
  description: string | null;
  archivedAt: string | null;
  createdAt: string;
  updatedAt: string;
};
export type APIKey = {
  id: string;
  environmentId: string;
  name: string;
  keyPrefix: string;
  lastUsedAt: string | null;
  expiresAt: string | null;
  createdAt: string;
};
export type AuditEntry = {
  id: string;
  actorId: string | null;
  actorType: "user" | "api_key" | "system";
  action: string;
  resourceType: string;
  resourceId: string | null;
  changes: unknown;
  ipAddress: string | null;
  userAgent: string | null;
  createdAt: string;
};
