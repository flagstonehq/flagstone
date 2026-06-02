import { cookies } from "next/headers";
import { NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080";

export async function PATCH(request: Request) {
  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  if (!token) {
    return NextResponse.json({ error: { code: "UNAUTHORIZED", message: "Not authenticated." } }, { status: 401 });
  }

  const body = await request.json();

  const backendRes = await fetch(`${BACKEND_URL}/api/v1/auth/me/password`, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(body),
  });

  if (!backendRes.ok) {
    const data = await backendRes.json();
    return NextResponse.json(data, { status: backendRes.status });
  }

  cookieStore.delete("access_token");
  return NextResponse.json({ ok: true, message: "Password updated. Please sign in again." });
}
