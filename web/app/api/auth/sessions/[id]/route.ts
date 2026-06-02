import { cookies } from "next/headers";
import { NextResponse } from "next/server";

const BACKEND_URL = process.env.BACKEND_URL ?? "http://localhost:8080";

export async function DELETE(_request: Request, { params }: { params: Promise<{ id: string }> }) {
  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;
  if (!token) {
    return NextResponse.json({ error: { code: "UNAUTHORIZED", message: "Not authenticated." } }, { status: 401 });
  }

  const { id } = await params;

  const backendRes = await fetch(`${BACKEND_URL}/api/v1/auth/sessions/${id}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!backendRes.ok) {
    const data = await backendRes.json();
    return NextResponse.json(data, { status: backendRes.status });
  }

  return new Response(null, { status: 204 });
}
