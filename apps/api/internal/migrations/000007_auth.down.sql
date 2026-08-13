-- 000007: auth (roles + server-side sessions)

DROP INDEX IF EXISTS idx_auth_sessions_expires_at;
DROP INDEX IF EXISTS idx_auth_sessions_user_id;
DROP TABLE IF EXISTS auth_sessions;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users DROP COLUMN IF EXISTS role;
