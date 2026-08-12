"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ApiError, listServiceRequests } from "../../lib/api";
import type { ServiceRequest } from "../../lib/types/api";
import { formatDate, shortId } from "../../lib/format";
import { Button } from "../../components/Button";
import { Card } from "../../components/Card";
import { PageHeader } from "../../components/PageHeader";
import { PriorityBadge } from "../../components/PriorityBadge";
import { StatusBadge } from "../../components/StatusBadge";

export default function RequestsPage() {
  const router = useRouter();
  const [requests, setRequests] = useState<ServiceRequest[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setRequests(null);
    setError(null);
    try {
      const data = await listServiceRequests();
      setRequests(data.requests);
    } catch (err) {
      setError(
        err instanceof ApiError ? err.message : "Unable to load requests.",
      );
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <>
      <PageHeader
        title="Service Requests"
        description="Create and manage service requests."
      >
        <Link href="/requests/new">
          <Button variant="primary">New Request</Button>
        </Link>
      </PageHeader>

      {error ? (
        <Card>
          <p className="alert alert--error">{error}</p>
          <Button variant="secondary" onClick={load}>
            Retry
          </Button>
        </Card>
      ) : null}

      {requests === null && !error ? (
        <Card>
          <p className="loading">Loading requests…</p>
        </Card>
      ) : null}

      {requests !== null && requests.length === 0 && !error ? (
        <Card>
          <div className="empty-state">
            <p>No service requests yet.</p>
            <Link href="/requests/new">
              <Button variant="primary">Create your first request</Button>
            </Link>
          </div>
        </Card>
      ) : null}

      {requests !== null && requests.length > 0 ? (
        <Card className="table-card">
          <table className="table">
            <thead>
              <tr>
                <th>Title</th>
                <th>Equipment</th>
                <th>Priority</th>
                <th>Status</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              {requests.map((request) => (
                <tr
                  key={request.id}
                  className="table__row"
                  onClick={() => router.push(`/requests/${request.id}`)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      router.push(`/requests/${request.id}`);
                    }
                  }}
                  tabIndex={0}
                >
                  <td>
                    <span className="table__title">{request.title}</span>
                    <span className="table__sub">{shortId(request.id)}</span>
                  </td>
                  <td>{shortId(request.equipment_id)}</td>
                  <td>
                    <PriorityBadge priority={request.priority} />
                  </td>
                  <td>
                    <StatusBadge status={request.status} />
                  </td>
                  <td>{formatDate(request.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      ) : null}
    </>
  );
}
