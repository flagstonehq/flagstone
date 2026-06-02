import { cookies } from "next/headers";
import { NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080";

export async function GET() {
  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  if (!token) {
    return NextResponse.json({ error: { code: "UNAUTHORIZED", message: "Not authenticated." } }, { status: 401 });
  }

  const backendRes = await fetch(`${BACKEND_URL}/api/v1/auth/me`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  const data = await backendRes.json();
  return NextResponse.json(data, { status: backendRes.status });
}
