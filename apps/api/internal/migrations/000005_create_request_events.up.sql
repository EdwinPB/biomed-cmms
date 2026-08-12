-- 000005: request_events

ALTER TABLE service_requests ADD CONSTRAINT service_requests_id_tenant_key UNIQUE (id, tenant_id);

CREATE TABLE request_events (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    request_id  uuid        NOT NULL,
    actor_id    uuid        NOT NULL,
    from_status text        NOT NULL CHECK (from_status IN ('pending', 'assigned', 'in_progress', 'resolved', 'cancelled')),
    to_status   text        NOT NULL CHECK (to_status IN ('pending', 'assigned', 'in_progress', 'resolved', 'cancelled')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, request_id) REFERENCES service_requests (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, actor_id)   REFERENCES users (tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_request_events_tenant_request    ON request_events (tenant_id, request_id);
CREATE INDEX idx_request_events_tenant_created_at ON request_events (tenant_id, created_at);
