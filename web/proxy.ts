import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const PUBLIC_PATHS = ["/login", "/setup", "/api/auth"];

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080";

export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const isPublic = PUBLIC_PATHS.some((p) => pathname.startsWith(p));
  const token = request.cookies.get("access_token")?.value;

  if (pathname === "/setup") {
    try {
      const res = await fetch(`${BACKEND_URL}/api/v1/setup/status`, { cache: "no-store" });
      const { initialized } = await res.json();
      if (initialized) {
        return NextResponse.redirect(new URL("/login", request.url));
      }
    } catch {
      // Backend unreachable — let the setup page handle it
    }
  }

  if (!isPublic && !token) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  if (isPublic && token && !pathname.startsWith("/api/auth")) {
    return NextResponse.redirect(new URL("/projects", request.url));
  }

  // Inject the access token as a Bearer header so the Go backend
  // receives it even on client-initiated requests (which only carry the
  // httpOnly cookie, not an Authorization header).
  if (token && pathname.startsWith("/api/v1/")) {
    const requestHeaders = new Headers(request.headers);
    requestHeaders.set("Authorization", `Bearer ${token}`);
    return NextResponse.next({ request: { headers: requestHeaders } });
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
