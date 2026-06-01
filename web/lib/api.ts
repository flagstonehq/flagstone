import { Environment, Project, APIKey } from "./types";

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
  return res.json() as Promise<T>;
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
  return res.json() as Promise<T>;
}
export function getProjects() {
  return apiFetch<{ projects: import("./types").Project[] }>("/api/v1/projects");
}
export function getFlags(projectSlug: string) {
  return apiFetch<{ flags: import("./types").Flag[] }>(`/api/v1/projects/${projectSlug}/flags`);
}
export function getEnvironments(projectSlug: string) {
  return apiFetch<{ environments: import("./types").Environment[] }>(
    `/api/v1/projects/${projectSlug}/environments`,
  );
}
export function getApiKeys(projectSlug: string, envId: string) {
  return apiFetch<{ api_keys: import("./types").APIKey[] }>(
    `/api/v1/projects/${projectSlug}/environments/${envId}/api-keys`,
  );
}
export function getAuditLog(projectSlug: string, params?: Record<string, string>) {
  const qs = params ? "?" + new URLSearchParams(params) : "";
  return apiFetch<{ entries: import("./types").AuditEntry[] }>(
    `/api/v1/projects/${projectSlug}/audit${qs}`,
  );
}

export function updateProject(projectSlug: string, data: { name: string }) {
  return apiFetch<{ project: Project }>(`/api/v1/projects/${projectSlug}`, {
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
  return apiFetch<{ environment: Environment }>(`/api/v1/projects/${projectSlug}/environments`, {
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
  return apiFetch<{ segments: import("./types").Segment[] }>(
    `/api/v1/projects/${projectSlug}/segments`,
  );
}

export function createProject(data: { name: string; slug: string }) {
  return apiFetch<{ project: import("./types").Project }>("/api/v1/projects", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export function createFlag(
  projectSlug: string,
  data: { key: string; name: string; type: string; description?: string },
) {
  return apiFetch<{ flag: import("./types").Flag }>(`/api/v1/projects/${projectSlug}/flags`, {
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
  return apiFetch<{ api_keys: APIKey[] }>(`/api/v1/projects/${projectSlug}/api-keys`);
}

export function createApiKey(
  projectSlug: string,
  data: { name: string; environment_id: string; expires_at?: string },
) {
  return apiFetch<{ api_key: APIKey & { raw_key: string } }>(
    `/api/v1/projects/${projectSlug}/api-keys`,
    { method: "POST", body: JSON.stringify(data) },
  );
}

export function revokeApiKey(projectSlug: string, envId: string, keyId: string) {
  return apiFetch<void>(`/api/v1/projects/${projectSlug}/environments/${envId}/api-keys/${keyId}`, {
    method: "DELETE",
  });
}
