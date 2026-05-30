import { cookies } from "next/headers"
import { NextRequest, NextResponse } from "next/server"

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080"

// The backend refresh endpoint reads the refresh_token httpOnly cookie that
// the Go server sets on login. We forward every cookie from the client so the
// backend can validate the refresh token without us ever touching its value.
export async function POST(request: NextRequest) {
  const cookieStore = await cookies()

  // Forward all cookies (refresh_token + any others) to the backend.
  const cookieHeader = request.headers.get("cookie") ?? ""

  const backendRes = await fetch(`${BACKEND_URL}/api/v1/auth/refresh`, {
    method: "POST",
    headers: { Cookie: cookieHeader },
  })

  if (!backendRes.ok) {
    cookieStore.delete("access_token")
    return NextResponse.json({ error: "Session expired" }, { status: 401 })
  }

  const data = await backendRes.json()

  cookieStore.set("access_token", data.access_token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    maxAge: data.expires_in ?? 900,
    path: "/",
  })

  return NextResponse.json({ ok: true })
}
