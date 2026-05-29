import { http, HttpResponse } from "msw";
const BACKEND = "http://localhost:8080";
export const handlers = [
  // Auth
  http.post(`${BACKEND}/api/v1/auth/login`, () =>
    HttpResponse.json({
      access_token: "test-jwt-token",
      token_type: "Bearer",
      expires_in: 900,
      tenant: { id: "t1", slug: "my-app", role: "owner" },
    }),
  ),
  // Projects
  http.get(`${BACKEND}/api/v1/projects`, () =>
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
  // Envs
  http.get(`${BACKEND}/api/v1/projects/:slug/environments`, () =>
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
  http.get(`${BACKEND}/api/v1/projects/:slug/flags`, () =>
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
];
