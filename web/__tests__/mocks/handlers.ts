import { http, HttpResponse } from "msw";

export const API_BASE = "http://localhost";
export const handlers = [
  // Auth
  http.post(`${API_BASE}/api/v1/auth/login`, () =>
    HttpResponse.json({
      access_token: "test-jwt-token",
      token_type: "Bearer",
      expires_in: 900,
      tenant: { id: "t1", slug: "my-app", role: "owner" },
    }),
  ),
  // Projects
  http.get(`${API_BASE}/api/v1/projects`, () =>
    HttpResponse.json({
      projects: [
        {
          id: "p1",
          tenant_id: "t1",
          slug: "my-app",
          name: "My App",
          created_at: new Date(Date.now() - 172800000).toISOString(),
          updated_at: new Date().toISOString(),
        },
        {
          id: "p2",
          tenant_id: "t1",
          slug: "backend-api",
          name: "Backend API",
          created_at: new Date(Date.now() - 604800000).toISOString(),
          updated_at: new Date().toISOString(),
        },
      ],
    }),
  ),
  // Create project
  http.post(`${API_BASE}/api/v1/projects`, async ({ request }) => {
    const body = (await request.json()) as { name: string; slug: string };
    if (body.slug === "duplicate-slug") {
      return HttpResponse.json(
        { error: { code: "SLUG_CONFLICT", message: "Slug already exists" } },
        { status: 409 },
      );
    }
    return HttpResponse.json(
      {
        project: {
          id: "p-new",
          tenant_id: "t1",
          slug: body.slug,
          name: body.name,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
      },
      { status: 201 },
    );
  }),
  // Single project
  http.get(`${API_BASE}/api/v1/projects/:slug`, ({ params }) => {
    const { slug } = params;
    if (slug === "my-app") {
      return HttpResponse.json({
        project: {
          id: "p1",
          tenant_id: "t1",
          slug: "my-app",
          name: "My App",
          created_at: new Date(Date.now() - 172800000).toISOString(),
          updated_at: new Date().toISOString(),
        },
      });
    }
    return HttpResponse.json(
      { error: { code: "NOT_FOUND", message: "Project not found" } },
      { status: 404 },
    );
  }),
  // Update project
  http.patch(`${API_BASE}/api/v1/projects/:slug`, async ({ request, params }) => {
    const body = (await request.json()) as { name: string };
    return HttpResponse.json({
      project: {
        id: "p1",
        tenant_id: "t1",
        slug: params.slug,
        name: body.name,
        created_at: new Date(Date.now() - 172800000).toISOString(),
        updated_at: new Date().toISOString(),
      },
    });
  }),
  // Delete project
  http.delete(`${API_BASE}/api/v1/projects/:slug`, () => new HttpResponse(null, { status: 204 })),
  // Envs
  http.get(`${API_BASE}/api/v1/projects/:slug/environments`, () =>
    HttpResponse.json({
      environments: [
        {
          id: "e1",
          project_id: "p1",
          slug: "development",
          name: "Development",
        },
        { id: "e2", project_id: "p1", slug: "staging", name: "Staging" },
        { id: "e3", project_id: "p1", slug: "production", name: "Production" },
      ],
    }),
  ),
  // Flags
  http.get(`${API_BASE}/api/v1/projects/:slug/flags`, () =>
    HttpResponse.json({
      flags: [
        {
          id: "f1",
          project_id: "p1",
          key: "new-checkout",
          name: "New Checkout Flow",
          type: "boolean",
          created_at: new Date().toISOString(),
        },
        {
          id: "f2",
          project_id: "p1",
          key: "dark-mode",
          name: "Dark Mode",
          type: "boolean",
          created_at: new Date().toISOString(),
        },
      ],
    }),
  ),
  // Toggle flag env
  http.patch(
    `${API_BASE}/api/v1/projects/:slug/flags/:key/environments/:env`,
    () => new HttpResponse(null, { status: 204 }),
  ),
  // Archive flag
  http.delete(
    `${API_BASE}/api/v1/projects/:slug/flags/:key`,
    () => new HttpResponse(null, { status: 204 }),
  ),
  // Create flag
  http.post(`${API_BASE}/api/v1/projects/:slug/flags`, async ({ request }) => {
    const body = (await request.json()) as { key: string; name: string; type: string };
    if (body.key === "duplicate-key") {
      return HttpResponse.json(
        { error: { code: "KEY_CONFLICT", message: "Flag key already exists" } },
        { status: 409 },
      );
    }
    return HttpResponse.json(
      {
        flag: {
          id: "f-new",
          project_id: "p1",
          key: body.key,
          name: body.name,
          type: body.type,
          description: null,
          archived_at: null,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
      },
      { status: 201 },
    );
  }),
  // Create environment
  http.post(`${API_BASE}/api/v1/projects/:slug/environments`, async ({ request }) => {
    const body = (await request.json()) as { name: string; slug: string };
    if (body.slug === "duplicate") {
      return HttpResponse.json(
        { error: { code: "SLUG_CONFLICT", message: "Slug already exists" } },
        { status: 409 },
      );
    }
    return HttpResponse.json(
      {
        environment: {
          id: "e-new",
          project_id: "p1",
          slug: body.slug,
          name: body.name,
        },
      },
      { status: 201 },
    );
  }),
  // Delete environment
  http.delete(
    `${API_BASE}/api/v1/projects/:slug/environments/:envId`,
    () => new HttpResponse(null, { status: 204 }),
  ),
  // API Keys — project-level listing
  http.get(`${API_BASE}/api/v1/projects/:slug/api-keys`, ({ params }) => {
    const { slug } = params;
    if (slug === "my-app") {
      return HttpResponse.json({
        api_keys: [
          {
            id: "ak1",
            environment_id: "e1",
            name: "Development Key",
            key_prefix: "fs_dev_abc",
            last_used_at: new Date().toISOString(),
            expires_at: null,
            created_at: new Date(Date.now() - 604800000).toISOString(),
          },
          {
            id: "ak2",
            environment_id: "e3",
            name: "Production Key",
            key_prefix: "fs_live_xyz",
            last_used_at: null,
            expires_at: new Date(Date.now() + 86400000 * 30).toISOString(),
            created_at: new Date(Date.now() - 86400000).toISOString(),
          },
        ],
      });
    }
    return HttpResponse.json({ api_keys: [] });
  }),
  // Create API Key
  http.post(`${API_BASE}/api/v1/projects/:slug/api-keys`, async ({ request }) => {
    const body = (await request.json()) as {
      name: string;
      environment_id: string;
    };
    return HttpResponse.json(
      {
        api_key: {
          id: "ak-new",
          environment_id: body.environment_id,
          name: body.name,
          key_prefix: "fs_live_new",
          last_used_at: null,
          expires_at: null,
          created_at: new Date().toISOString(),
          raw_key: "fs_live_new_abc123def456ghi789",
        },
      },
      { status: 201 },
    );
  }),
  // Revoke API Key
  http.delete(
    `${API_BASE}/api/v1/projects/:slug/environments/:envId/api-keys/:keyId`,
    () => new HttpResponse(null, { status: 204 }),
  ),
  // Flag Environment — get
  http.get(`${API_BASE}/api/v1/projects/:slug/flags/:key/environments/:env`, ({ params }) => {
    const { slug, key, env } = params;
    if (slug === "my-app" && key === "new-checkout" && env === "production") {
      return HttpResponse.json({
        flag_id: "f1",
        environment_id: "e3",
        enabled: true,
        rules: [
          {
            conditions: {
              all: [
                { attribute: "country", operator: "eq", value: "AR" },
              ],
            },
            value: true,
          },
        ],
        default_value: false,
        version: 5,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      });
    }
    return HttpResponse.json({
      flag_id: "f1",
      environment_id: env === "e1" ? "e1" : "e3",
      enabled: false,
      rules: [],
      default_value: false,
      version: 1,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    });
  }),
  // Flag Environment — save
  http.put(`${API_BASE}/api/v1/projects/:slug/flags/:key/environments/:env`, async ({ request, params }) => {
    const { slug, env } = params;
    const body = (await request.json()) as { version?: number } & Record<string, unknown>;
    if (body.version !== undefined && body.version < 5 && slug === "my-app") {
      return HttpResponse.json(
        { error: { code: "VERSION_CONFLICT", message: "The flag environment configuration was modified by another request. Reload and retry." } },
        { status: 409 },
      );
    }
    return HttpResponse.json({
      flag_id: "f1",
      environment_id: env === "e1" ? "e1" : "e3",
      enabled: body.enabled ?? false,
      rules: body.rules ?? [],
      default_value: false,
      version: (body.version as number) + 1,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    });
  }),
  // Audit
  http.get(`${API_BASE}/api/v1/audit`, ({ request }) => {
    const url = new URL(request.url);
    const actionFilter = url.searchParams.get("action");
    const resourceFilter = url.searchParams.get("resource_type");
    const actorTypeFilter = url.searchParams.get("actor_type");
    const limit = Math.min(100, Number(url.searchParams.get("limit") ?? 20));
    const offset = Number(url.searchParams.get("offset") ?? 0);
    const all = [
      {
        id: "00000000-0000-4000-a000-000000000001",
        actor_id: "00000000-0000-4000-a000-0000000000a1",
        actor_type: "user",
        action: "project.created",
        resource_type: "project",
        resource_id: "00000000-0000-4000-a000-0000000000b1",
        created_at: new Date(Date.now() - 60_000).toISOString(),
      },
      {
        id: "00000000-0000-4000-a000-000000000002",
        actor_id: "00000000-0000-4000-a000-0000000000a1",
        actor_type: "user",
        action: "flag.created",
        resource_type: "flag",
        resource_id: "00000000-0000-4000-a000-0000000000b2",
        created_at: new Date(Date.now() - 300_000).toISOString(),
      },
      {
        id: "00000000-0000-4000-a000-000000000003",
        actor_id: "00000000-0000-4000-a000-0000000000a3",
        actor_type: "api_key",
        action: "flag.toggled",
        resource_type: "flag",
        resource_id: "00000000-0000-4000-a000-0000000000b2",
        created_at: new Date(Date.now() - 3_600_000).toISOString(),
      },
      {
        id: "00000000-0000-4000-a000-000000000004",
        actor_id: null,
        actor_type: "system",
        action: "flag.archived",
        resource_type: "flag",
        resource_id: "00000000-0000-4000-a000-0000000000b3",
        created_at: new Date(Date.now() - 86_400_000).toISOString(),
      },
    ];
    let filtered = all;
    if (actionFilter) filtered = filtered.filter((e) => e.action === actionFilter);
    if (resourceFilter) filtered = filtered.filter((e) => e.resource_type === resourceFilter);
    if (actorTypeFilter) filtered = filtered.filter((e) => e.actor_type === actorTypeFilter);
    const total = filtered.length;
    const entries = filtered.slice(offset, offset + limit);
    return HttpResponse.json({ entries, total, limit, offset });
  }),
];
