-- 000003: equipment

CREATE TABLE equipment (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    asset_tag     text        NOT NULL,
    name          text        NOT NULL,
    serial_number text        NOT NULL,
    location      text        NOT NULL DEFAULT '',
    status        text        NOT NULL DEFAULT 'operational'
                              CHECK (status IN ('operational', 'maintenance', 'retired')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_equipment_tenant_asset_tag ON equipment (tenant_id, asset_tag);
CREATE INDEX         idx_equipment_tenant_id       ON equipment (tenant_id);

CREATE TRIGGER equipment_set_updated_at
BEFORE UPDATE ON equipment
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
