import { Badge } from "./Badge";
import type { RequestPriority } from "../lib/types/api";

const PRIORITY_TONE: Record<RequestPriority, "neutral" | "ok" | "warn" | "bad"> = {
  low: "neutral",
  medium: "ok",
  high: "warn",
  critical: "bad",
};

const PRIORITY_LABEL: Record<RequestPriority, string> = {
  low: "Low",
  medium: "Medium",
  high: "High",
  critical: "Critical",
};

export function PriorityBadge({ priority }: { priority: RequestPriority }) {
  return (
    <Badge tone={PRIORITY_TONE[priority]}>{PRIORITY_LABEL[priority]}</Badge>
  );
}
