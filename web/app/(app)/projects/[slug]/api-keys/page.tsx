import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { serverFetch } from "@/lib/api";
import type { APIKey, Environment } from "@/lib/types";
import { Topbar } from "@/components/layout/topbar";
import { ApiKeysTable } from "@/components/api-keys/api-keys-table";
import { CreateKeyDialog } from "@/components/api-keys/create-key-dialog";

interface ApiKeysPaegProps {
  params: Promise<{ slug: string }>;
}

export default async function ApiKeyPage({ params }: ApiKeysPaegProps) {
  const { slug } = await params;
  const cookieStore = await cookies();
  const token = cookieStore.get("access_token")?.value;

  if (!token) redirect("/login");

  let apiKeys: APIKey[] = [];
  let environments: Environment[] = [];
  try {
    environments = await serverFetch<Environment[]>(
      `/api/v1/projects/${slug}/environments`,
      token,
    );
    const envSlug = environments[0]?.slug ?? "production";
    apiKeys = await serverFetch<APIKey[]>(
      `/api/v1/projects/${slug}/environments/${envSlug}/apikeys`,
      token,
    );
  } catch {
    // Backend endpoints may not be available yet
  }

  return (
    <div className="flex h-full flex-col">
      <Topbar
        title="API Keys"
        action={<CreateKeyDialog projectSlug={slug} environments={environments} />}
      />
      <main className="flex-1 overflow-y-auto p-4 sm:p-6">
        <ApiKeysTable apiKeys={apiKeys} environments={environments} projectSlug={slug} />
      </main>
    </div>
  );
}
