DROP TRIGGER service_requests_set_updated_at ON service_requests;
DROP TABLE service_requests;
ALTER TABLE equipment DROP CONSTRAINT equipment_id_tenant_key;
ALTER TABLE users      DROP CONSTRAINT users_id_tenant_key;
