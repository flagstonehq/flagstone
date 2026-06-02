export class ApiError extends Error {
  constructor(
    public readonly code: string,
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080";

function snakeToCamel(str: string): string {
  return str.replace(/_([a-z])/g, (_, letter: string) => letter.toUpperCase());
}

export function transformKeys<T>(obj: unknown): T {
  if (Array.isArray(obj)) {
    return obj.map((item) => transformKeys(item)) as T;
  }
  if (obj !== null && typeof obj === "object") {
    const result: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(obj)) {
      result[snakeToCamel(key)] = transformKeys(value);
    }
    return result as T;
  }
  return obj as T;
}

export async function serverFetch<T>(path: string, token: string): Promise<T> {
  const res = await fetch(`${BACKEND_URL}${path}`, {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    cache: "no-store",
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new ApiError(
      body?.error?.code ?? "UNKNOWN",
      res.status,
      body?.error?.message ?? res.statusText,
    );
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json();
  return transformKeys<T>(data);
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new ApiError(
      body?.error?.code ?? "UNKNOWN",
      res.status,
      body?.error?.message ?? res.statusText,
    );
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json();
  return transformKeys<T>(data);
}
export function getProjects() {
  return apiFetch<import("./types").Project[]>("/api/v1/projects");
}
export function getFlags(projectSlug: string) {
  return apiFetch<import("./types").Flag[]>(`/api/v1/projects/${projectSlug}/flags`);
}
export function getEnvironments(projectSlug: string) {
  return apiFetch<import("./types").Environment[]>(
    `/api/v1/projects/${projectSlug}/environments`,
  );
}
export function getApiKeys(projectSlug: string, envId: string) {
  return apiFetch<import("./types").APIKey[]>(
    `/api/v1/projects/${projectSlug}/environments/${envId}/apikeys`,
  );
}
export type AuditLogParams = {
  actor_type?: "user" | "api_key" | "system";
  action?: string;
  resource_type?: string;
  since?: string;
  until?: string;
  limit?: number;
  offset?: number;
};
export type AuditLogPage = {
  entries: import("./types").AuditEntry[];
  total: number;
  limit: number;
  offset: number;
};
export function getAuditLog(params?: AuditLogParams) {
  const search = new URLSearchParams();
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== "") {
        search.set(key, String(value));
      }
    }
  }
  const qs = search.toString();
  return apiFetch<AuditLogPage>(`/api/v1/audit${qs ? "?" + qs : ""}`);
}

export function updateProject(projectSlug: string, data: { name: string }) {
  return apiFetch<import("./types").Project>(`/api/v1/projects/${projectSlug}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}

export function deleteProject(projectSlug: string) {
  return apiFetch<void>(`/api/v1/projects/${projectSlug}`, {
    method: "DELETE",
  });
}

export function createEnvironment(projectSlug: string, data: { name: string; slug: string }) {
  return apiFetch<import("./types").Environment>(`/api/v1/projects/${projectSlug}/environments`, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export function deleteEnvironment(projectSlug: string, envId: string) {
  return apiFetch<void>(`/api/v1/projects/${projectSlug}/environments/${envId}`, {
    method: "DELETE",
  });
}

export function getSegments(projectSlug: string) {
  return apiFetch<import("./types").Segment[]>(
    `/api/v1/projects/${projectSlug}/segments`,
  );
}
export function getSegment(projectSlug: string, key: string) {
  return apiFetch<import("./types").Segment>(
    `/api/v1/projects/${projectSlug}/segments/${key}`,
  );
}
export function createSegment(
  projectSlug: string,
  data: { key: string; name: string; description?: string; rules?: unknown },
) {
  return apiFetch<import("./types").Segment>(
    `/api/v1/projects/${projectSlug}/segments`,
    { method: "POST", body: JSON.stringify(data) },
  );
}
export function updateSegment(
  projectSlug: string,
  key: string,
  data: { name?: string; description?: string; rules?: unknown },
) {
  return apiFetch<import("./types").Segment>(
    `/api/v1/projects/${projectSlug}/segments/${key}`,
    { method: "PUT", body: JSON.stringify(data) },
  );
}
export function archiveSegment(projectSlug: string, key: string) {
  return apiFetch<void>(`/api/v1/projects/${projectSlug}/segments/${key}`, {
    method: "DELETE",
  });
}

export function createProject(data: { name: string; slug: string }) {
  return apiFetch<import("./types").Project>("/api/v1/projects", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export function createFlag(
  projectSlug: string,
  data: { key: string; name: string; type: string; description?: string },
) {
  return apiFetch<import("./types").Flag>(`/api/v1/projects/${projectSlug}/flags`, {
    method: "POST",
    body: JSON.stringify(data),
  });
}
export function archiveFlag(projectSlug: string, flagKey: string) {
  return apiFetch<void>(`/api/v1/projects/${projectSlug}/flags/${flagKey}`, { method: "DELETE" });
}
export function toggleFlagEnv(
  projectSlug: string,
  flagKey: string,
  envSlug: string,
  enabled: boolean,
) {
  return apiFetch<void>(
    `/api/v1/projects/${projectSlug}/flags/${flagKey}/environments/${envSlug}`,
    { method: "PATCH", body: JSON.stringify({ enabled }) },
  );
}

export function getProjectApiKeys(projectSlug: string) {
  return apiFetch<import("./types").APIKey[]>(`/api/v1/projects/${projectSlug}/api-keys`);
}

export function createApiKey(
  projectSlug: string,
  data: { name: string; environment_id: string; expires_at?: string },
) {
  return apiFetch<import("./types").APIKey & { rawKey: string }>(
    `/api/v1/projects/${projectSlug}/api-keys`,
    { method: "POST", body: JSON.stringify(data) },
  );
}

export function revokeApiKey(projectSlug: string, envId: string, keyId: string) {
  return apiFetch<void>(`/api/v1/projects/${projectSlug}/environments/${envId}/api-keys/${keyId}`, {
    method: "DELETE",
  });
}
export function getFlagStates(projectSlug: string, envSlug: string) {
  return apiFetch<{ flagKey: string; enabled: boolean }[]>(
    `/api/v1/projects/${projectSlug}/environments/${envSlug}/flag-states`,
  );
}

export function getFlag(projectSlug: string, flagKey: string) {
  return apiFetch<import("./types").Flag>(
    `/api/v1/projects/${projectSlug}/flags/${flagKey}`,
  );
}
export function getFlagEnvironment(projectSlug: string, flagKey: string, envSlug: string) {
  return apiFetch<import("./types").FlagEnvironmentConfig>(
    `/api/v1/projects/${projectSlug}/flags/${flagKey}/environments/${envSlug}`,
  );
}
export function getMe() {
  return apiFetch<import("./types").User>("/api/auth/me");
}
export function changePassword(data: { current_password: string; new_password: string; confirm_password: string }) {
  return apiFetch<void>("/api/v1/auth/me/password", {
    method: "PATCH",
    body: JSON.stringify(data),
  });
}
export function getSessions() {
  return apiFetch<import("./types").Session[]>("/api/v1/auth/sessions");
}
export function revokeSession(id: string) {
  return apiFetch<void>(`/api/v1/auth/sessions/${id}`, { method: "DELETE" });
}
export function revokeAllSessions() {
  return apiFetch<void>("/api/v1/auth/sessions", { method: "DELETE" });
}
export function saveFlagEnvironment(
  projectSlug: string,
  flagKey: string,
  envSlug: string,
  data: { enabled: boolean; rules: import("./types").Rule[]; version: number },
) {
  return apiFetch<import("./types").FlagEnvironmentConfig>(
    `/api/v1/projects/${projectSlug}/flags/${flagKey}/environments/${envSlug}`,
    { method: "PUT", body: JSON.stringify(data) },
  );
}
