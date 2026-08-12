"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ApiError, getRequestHistory, getServiceRequest } from "../../../lib/api";
import type {
  RequestEvent,
  RequestStatus,
  ServiceRequest,
} from "../../../lib/types/api";
import { formatDate, shortId } from "../../../lib/format";
import { Button } from "../../../components/Button";
import { Card } from "../../../components/Card";
import { PageHeader } from "../../../components/PageHeader";
import { PriorityBadge } from "../../../components/PriorityBadge";
import { RequestStatusControl } from "../../../components/RequestStatusControl";
import { StatusBadge } from "../../../components/StatusBadge";

export default function RequestDetailPage() {
  const params = useParams<{ id: string }>();
  const requestId = params.id;

  const [request, setRequest] = useState<ServiceRequest | null>(null);
  const [events, setEvents] = useState<RequestEvent[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notFound, setNotFound] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setNotFound(false);
    try {
      const [requestData, historyData] = await Promise.all([
        getServiceRequest(requestId),
        getRequestHistory(requestId),
      ]);
      setRequest(requestData);
      setEvents(historyData.events);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setNotFound(true);
      } else {
        setError(
          err instanceof ApiError
            ? err.message
            : "Unable to load the request.",
        );
      }
    } finally {
      setLoading(false);
    }
  }, [requestId]);

  useEffect(() => {
    load();
  }, [load]);

  const handleStatusChanged = useCallback(
    (status: RequestStatus) => {
      setRequest((prev) => (prev ? { ...prev, status } : prev));
      load();
    },
    [load],
  );

  if (notFound) {
    return (
      <>
        <PageHeader title="Request not found" />
        <Card>
          <div className="empty-state">
            <p>This service request does not exist or you lack access.</p>
            <Link href="/requests">
              <Button variant="secondary">Back to requests</Button>
            </Link>
          </div>
        </Card>
      </>
    );
  }

  if (loading && !request) {
    return <PageHeader title="Service Request" description="Loading request…" />;
  }

  if (error && !request) {
    return (
      <>
        <PageHeader title="Service Request" />
        <Card>
          <p className="alert alert--error">{error}</p>
          <Button variant="secondary" onClick={load}>
            Retry
          </Button>
        </Card>
      </>
    );
  }

  if (!request) {
    return null;
  }

  return (
    <>
      <PageHeader title={request.title} description={shortId(request.id)}>
        <Link href="/requests">
          <Button variant="secondary">Back to requests</Button>
        </Link>
      </PageHeader>

      <Card title="Details">
        <div className="detail-grid">
          <div className="detail-field">
            <span className="detail-field__label">Equipment</span>
            <span className="detail-field__value">{shortId(request.equipment_id)}</span>
          </div>
          <div className="detail-field">
            <span className="detail-field__label">Priority</span>
            <span className="detail-field__value">
              <PriorityBadge priority={request.priority} />
            </span>
          </div>
          <div className="detail-field">
            <span className="detail-field__label">Status</span>
            <span className="detail-field__value">
              <StatusBadge status={request.status} />
            </span>
          </div>
          <div className="detail-field">
            <span className="detail-field__label">Created by</span>
            <span className="detail-field__value">{shortId(request.created_by)}</span>
          </div>
          <div className="detail-field">
            <span className="detail-field__label">Created</span>
            <span className="detail-field__value">{formatDate(request.created_at)}</span>
          </div>
          <div className="detail-field">
            <span className="detail-field__label">Updated</span>
            <span className="detail-field__value">{formatDate(request.updated_at)}</span>
          </div>
        </div>

        <div className="detail-field detail-field--block">
          <span className="detail-field__label">Description</span>
          <span className="detail-field__value">{request.description}</span>
        </div>
      </Card>

      <Card title="Update status">
        <RequestStatusControl
          requestId={request.id}
          currentStatus={request.status}
          onStatusChanged={handleStatusChanged}
        />
        {request.status === "resolved" || request.status === "cancelled" ? (
          <p className="detail-note">
            This request is closed. No further status changes are allowed.
          </p>
        ) : null}
      </Card>

      <Card title="History">
        {events === null ? (
          <p className="loading">Loading history…</p>
        ) : events.length === 0 ? (
          <p className="empty-state">No activity yet for this request.</p>
        ) : (
          <ol className="timeline">
            {events.map((event) => (
              <li key={event.id} className="timeline__item">
                <div className="timeline__body">
                  <p className="timeline__change">
                    <StatusBadge status={event.from_status} />
                    <span className="timeline__arrow" aria-hidden="true">
                      →
                    </span>
                    <StatusBadge status={event.to_status} />
                  </p>
                  <p className="timeline__meta">
                    {shortId(event.actor_id)} · {formatDate(event.created_at)}
                  </p>
                </div>
              </li>
            ))}
          </ol>
        )}
      </Card>
    </>
  );
}
