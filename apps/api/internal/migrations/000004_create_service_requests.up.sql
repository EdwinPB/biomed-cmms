-- 000004: service_requests

-- Composite unique keys on the referenced tables let the FKs below enforce
-- that equipment/user references always belong to the same tenant.
ALTER TABLE equipment ADD CONSTRAINT equipment_id_tenant_key UNIQUE (id, tenant_id);
ALTER TABLE users      ADD CONSTRAINT users_id_tenant_key      UNIQUE (id, tenant_id);

CREATE TABLE service_requests (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    equipment_id     uuid        NOT NULL,
    title            text        NOT NULL,
    description      text        NOT NULL,
    priority         text        NOT NULL DEFAULT 'medium'
                                  CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    status           text        NOT NULL DEFAULT 'pending'
                                  CHECK (status IN ('pending', 'assigned', 'in_progress', 'resolved', 'cancelled')),
    created_by       uuid        NOT NULL,
    assigned_to      uuid,
    resolution_notes text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, equipment_id) REFERENCES equipment (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, created_by)   REFERENCES users      (tenant_id, id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, assigned_to)  REFERENCES users      (tenant_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_service_requests_tenant_status     ON service_requests (tenant_id, status);
CREATE INDEX idx_service_requests_tenant_created_at ON service_requests (tenant_id, created_at);
CREATE INDEX idx_service_requests_tenant_equipment  ON service_requests (tenant_id, equipment_id);

CREATE TRIGGER service_requests_set_updated_at
BEFORE UPDATE ON service_requests
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
