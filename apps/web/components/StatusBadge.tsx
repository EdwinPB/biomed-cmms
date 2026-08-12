import { Badge } from "./Badge";
import type { RequestStatus } from "../lib/types/api";

const STATUS_TONE: Record<RequestStatus, "neutral" | "ok" | "warn" | "bad" | "accent"> = {
  pending: "accent",
  assigned: "warn",
  in_progress: "accent",
  resolved: "ok",
  cancelled: "neutral",
};

const STATUS_LABEL: Record<RequestStatus, string> = {
  pending: "Pending",
  assigned: "Assigned",
  in_progress: "In progress",
  resolved: "Resolved",
  cancelled: "Cancelled",
};

export function StatusBadge({ status }: { status: RequestStatus }) {
  return <Badge tone={STATUS_TONE[status]}>{STATUS_LABEL[status]}</Badge>;
}

export { STATUS_LABEL };
