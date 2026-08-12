-- 000006: rfps

CREATE TABLE rfps (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    service_request_id uuid        NOT NULL,
    title              text        NOT NULL,
    description        text        NOT NULL,
    status             text        NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'published', 'closed', 'cancelled')),
    due_at             timestamptz,
    created_by         uuid        NOT NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, service_request_id) REFERENCES service_requests (tenant_id, id) ON DELETE CASCADE,
    FOREIGN KEY (tenant_id, created_by)         REFERENCES users (tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_rfps_tenant_service_request ON rfps (tenant_id, service_request_id);
CREATE INDEX idx_rfps_tenant_created_at      ON rfps (tenant_id, created_at);

-- An RFP is "active" while it is still in play: draft or published.
-- Closed and cancelled RFPs no longer count, so a fresh RFP can be created for
-- a service request once the previous one is closed or cancelled. This is a
-- partial index, not a blanket unique constraint.
CREATE UNIQUE INDEX idx_rfps_one_active_per_service_request
    ON rfps (tenant_id, service_request_id)
    WHERE status IN ('draft', 'published');
