"use client";

import { useState } from "react";
import { ApiError, transitionServiceRequest } from "../lib/api";
import type { RequestStatus } from "../lib/types/api";

const NEXT_ACTIONS: Partial<Record<RequestStatus, RequestStatus[]>> = {
  pending: ["assigned", "cancelled"],
  assigned: ["in_progress", "cancelled"],
  in_progress: ["resolved", "cancelled"],
};

const ACTION_LABEL: Record<RequestStatus, string> = {
  pending: "Reopen",
  assigned: "Assign",
  in_progress: "Start work",
  resolved: "Mark resolved",
  cancelled: "Cancel request",
};

type Props = {
  requestId: string;
  currentStatus: RequestStatus;
  onStatusChanged: (status: RequestStatus) => void;
};

export function RequestStatusControl({
  requestId,
  currentStatus,
  onStatusChanged,
}: Props) {
  const [pendingStatus, setPendingStatus] = useState<RequestStatus | null>(null);
  const [error, setError] = useState<string | null>(null);

  const actions = NEXT_ACTIONS[currentStatus] ?? [];
  const inFlight = pendingStatus !== null;

  async function handleTransition(to: RequestStatus) {
    setPendingStatus(to);
    setError(null);
    try {
      const updated = await transitionServiceRequest(requestId, { status: to });
      onStatusChanged(updated.status);
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.message
          : "Unable to update status. Please try again.",
      );
    } finally {
      setPendingStatus(null);
    }
  }

  if (actions.length === 0) {
    return null;
  }

  return (
    <div className="status-control">
      <div className="status-control__actions">
        {actions.map((to) => (
          <button
            key={to}
            type="button"
            className={`btn btn--secondary status-control__action ${
              to === "cancelled" ? "status-control__action--danger" : ""
            }`}
            disabled={inFlight}
            onClick={() => handleTransition(to)}
          >
            {pendingStatus === to ? "Updating…" : ACTION_LABEL[to]}
          </button>
        ))}
      </div>
      {error ? <p className="alert alert--error">{error}</p> : null}
    </div>
  );
}
