import { cookies } from "next/headers";
import { NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080";

export async function POST() {
  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;

  if (!token) {
    return NextResponse.json({ error: "No session" }, { status: 401 });
  }

  const backendRes = await fetch(`${BACKEND_URL}/api/v1/auth/refresh`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!backendRes.ok) {
    cookieStore.delete("access_token");
    return NextResponse.json({ error: "Session expired" }, { status: 401 });
  }

  const data = await backendRes.json();

  cookieStore.set("access_token", data.access_token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    maxAge: data.expires_in ?? 900,
    path: "/",
  });

  return NextResponse.json({ ok: true });
}
