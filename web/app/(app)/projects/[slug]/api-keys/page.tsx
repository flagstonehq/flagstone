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

  const [{ api_keys }, { environments }] = await Promise.all([
    serverFetch<{ api_keys: APIKey[] }>(`/api/v1/projects/${slug}/api-keys`, token),
    serverFetch<{ environments: Environment[] }>(`/api/v1/projects/${slug}/environments`, token),
  ]);

  return (
    <div className="flex h-full flex-col">
      <Topbar
        title="API Keys"
        action={<CreateKeyDialog projectSlug={slug} environments={environments} />}
      />
      <main className="flex-1 overflow-y-auto p-6">
        <ApiKeysTable apiKeys={api_keys} environments={environments} projectSlug={slug} />
      </main>
    </div>
  );
}
