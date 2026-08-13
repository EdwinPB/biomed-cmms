-- 000007: auth (roles + server-side sessions)

ALTER TABLE users
    ADD COLUMN role text NOT NULL DEFAULT 'requester',
    ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'biomedic', 'requester'));

CREATE TABLE auth_sessions (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash   text        NOT NULL UNIQUE,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    tenant_id    uuid        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    expires_at   timestamptz NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_auth_sessions_user_id    ON auth_sessions (user_id);
CREATE INDEX idx_auth_sessions_expires_at ON auth_sessions (expires_at);

-- Local development bootstrap: upgrade the existing dev user to a bcrypt
-- password and the admin role so it can log in through the auth flow.
UPDATE users u
SET password_hash = '$2a$10$AAZLkEqOuWupLfw.D5sFyuhAmW/CXDmJ5SehlpyNfaEZh1Xeoe3iO',
    role          = 'admin'
FROM tenants t
WHERE t.slug = 'local-dev'
  AND u.tenant_id = t.id
  AND u.email = 'dev@local.test';
