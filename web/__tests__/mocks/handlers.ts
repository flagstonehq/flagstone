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
];
