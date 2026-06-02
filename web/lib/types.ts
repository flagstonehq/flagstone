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
export type LeafCondition = {
  attribute: string;
  operator:
    | "eq" | "neq"
    | "gt" | "gte" | "lt" | "lte"
    | "in" | "not_in"
    | "contains" | "starts_with" | "ends_with" | "matches"
    | "exists" | "not_exists"
    | "segment";
  value: unknown;
};
export type RuleConditionNode =
  | LeafCondition
  | { all: RuleConditionNode[] }
  | { any: RuleConditionNode[] }
  | { not: RuleConditionNode };
export type RuleRollout = {
  percentage: number;
  seed?: string;
};
export type Rule = {
  conditions: RuleConditionNode;
  rollout?: RuleRollout;
  value: unknown;
};
export type FlagEnvironmentConfig = {
  flagId: string;
  environmentId: string;
  enabled: boolean;
  rules: Rule[];
  defaultValue: unknown;
  version: number;
  createdAt: string;
  updatedAt: string;
};
export type FlagEnvironment = {
  flagId: string;
  environmentId: string;
  enabled: boolean;
  rules: Rule[];
  version: number;
};
export type Segment = {
  id: string;
  projectId: string;
  key: string;
  name: string;
  description: string | null;
  rules: RuleConditionNode;
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
export type User = {
  id: string;
  email: string;
  role: "owner" | "admin" | "member" | "viewer";
  createdAt: string;
  lastLoginAt: string | null;
};
export type Session = {
  id: string;
  ipAddress: string | null;
  userAgent: string | null;
  createdAt: string;
  expiresAt: string;
  isCurrent: boolean;
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
