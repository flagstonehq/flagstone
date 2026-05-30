import { cookies } from "next/headers";
import { NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080";

export async function POST(request: Request) {
  const body = await request.json();

  const backendRes = await fetch(`${BACKEND_URL}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  const data = await backendRes.json();

  if (!backendRes.ok) {
    // Includes 409 MULTIPLE_TENANTS — pass the full response body so the
    // LoginForm can render the tenant picker from data.error.available_tenants.
    return NextResponse.json(data, { status: backendRes.status });
  }

  const cookieStore = await cookies();
  cookieStore.set("access_token", data.access_token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    maxAge: data.expires_in ?? 900,
    path: "/",
  });

  return NextResponse.json({ ok: true, tenant: data.tenant });
}
