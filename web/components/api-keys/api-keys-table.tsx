"use client";
import type { APIKey, Environment } from "@/lib/types";
import { formatDate, formatRelative } from "@/lib/utils";
import { RevokeKeyButton } from "@/components/api-keys/revoke-key-button";

interface ApiKeysTableProps {
  apiKeys: APIKey[];
  environments: Environment[];
  projectSlug: string;
}

function envName(envId: string, environments: Environment[]) {
  return environments.find((e) => e.id === envId)?.name ?? envId;
}

export function ApiKeysTable({ apiKeys, environments, projectSlug }: ApiKeysTableProps) {
  if (apiKeys.length === 0) {
    return (
      <p className="text-text-secondary py-12 text-center text-sm">
        No API keys yet. Create one to get started.
      </p>
    );
  }

  return (
    <div className="border-border overflow-x-auto rounded-lg border">
      <table className="w-full text-sm">
        <thead className="bg-muted/50 text-text-secondary text-left text-xs uppercase">
          <tr>
            <th className="px-4 py-3 font-medium">Name</th>
            <th className="px-4 py-3 font-medium">Key</th>
            <th className="px-4 py-3 font-medium">Environment</th>
            <th className="px-4 py-3 font-medium">Created</th>
            <th className="px-4 py-3 font-medium">Last used</th>
            <th className="px-4 py-3 font-medium">Expires</th>
            <th className="px-4 py-3 font-medium" />
          </tr>
        </thead>
        <tbody className="divide-border divide-y">
          {apiKeys.map((key) => (
            <tr key={key.id} className="group">
              <td className="text-text px-4 py-3 font-medium">{key.name}</td>
              <td className="text-text-secondary px-4 py-3 font-mono text-xs">{key.keyPrefix}…</td>
              <td className="text-text-secondary px-4 py-3">
                {envName(key.environmentId, environments)}
              </td>
              <td className="text-text-secondary px-4 py-3">{formatDate(key.createdAt)}</td>
              <td className="text-text-secondary px-4 py-3">
                {key.lastUsedAt ? formatRelative(key.lastUsedAt) : "—"}
              </td>
              <td className="text-text-secondary px-4 py-3">
                {key.expiresAt ? formatDate(key.expiresAt) : "—"}
              </td>
              <td className="px-4 py-3 text-right">
                <RevokeKeyButton
                  projectSlug={projectSlug}
                  envId={key.environmentId}
                  keyId={key.id}
                  keyName={key.name}
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
