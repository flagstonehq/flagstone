import {
  FolderPlus, Flag, Users, Key, Trash2, Pencil, ToggleLeft,
  Activity, type LucideIcon,
} from "lucide-react";
import { formatRelative, formatDate } from "@/lib/utils";
import type { AuditEntry } from "@/lib/types";

interface AuditRowProps {
  entry: AuditEntry;
}

const ACTION_ICONS: Record<string, LucideIcon> = {
  "project.created": FolderPlus,
  "project.updated": Pencil,
  "project.deleted": Trash2,
  "flag.created": Flag,
  "flag.updated": Pencil,
  "flag.archived": Trash2,
  "flag.toggled": ToggleLeft,
  "segment.created": Users,
  "segment.updated": Pencil,
  "segment.archived": Trash2,
  "environment.created": FolderPlus,
  "environment.deleted": Trash2,
  "api_key.created": Key,
  "api_key.revoked": Trash2,
};

function actionVerb(action: string): string {
  const [, verb] = action.split(".");
  if (!verb) return action;
  return verb.replace(/_/g, " ");
}

function resourceLabel(type: string): string {
  return type.replace(/_/g, " ");
}

function actorLabel(entry: AuditEntry): string {
  if (entry.actorType === "system") return "System";
  if (entry.actorType === "api_key") return "API key";
  if (entry.actorId) return entry.actorId.slice(0, 8);
  return "\u2014";
}

export function AuditRow({ entry }: AuditRowProps) {
  const Icon = ACTION_ICONS[entry.action] ?? Activity;
  return (
    <tr className="border-t border-border first:border-t-0">
      <td className="px-4 py-2.5">
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4 shrink-0 text-text-secondary" />
          <span className="font-medium capitalize">
            {resourceLabel(entry.resourceType)} {actionVerb(entry.action)}
          </span>
        </div>
      </td>
      <td className="px-4 py-2.5 text-text-secondary">{actorLabel(entry)}</td>
      <td className="px-4 py-2.5 text-text-secondary">
        {entry.resourceId ? entry.resourceId.slice(0, 8) : "\u2014"}
      </td>
      <td className="px-4 py-2.5 text-text-secondary" title={formatDate(entry.createdAt)}>
        {formatRelative(entry.createdAt)}
      </td>
    </tr>
  );
}
