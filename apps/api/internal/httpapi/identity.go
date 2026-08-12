package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// Request identity is established from development headers (X-Tenant-ID,
// X-User-ID) by the identity middleware and carried in the request context.
// The mechanism is isolated behind these helpers so it can later be replaced
// by real authentication middleware without touching business handlers.

type identityContextKey struct{}

type requestIdentity struct {
	tenantID uuid.UUID
	userID   uuid.UUID
}

// identity reads the development identity headers and injects them into the
// request context. X-Tenant-ID is required and must be a valid UUID. X-User-ID
// is optional; when present it must be a valid UUID. Failures return 401.
func (h *handler) identity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, err := parseUUIDHeader(r, "X-Tenant-ID")
		if err != nil {
			writeError(w, http.StatusUnauthorized, "missing or invalid X-Tenant-ID header")
			return
		}

		userID := uuid.Nil
		if raw := r.Header.Get("X-User-ID"); raw != "" {
			userID, err = uuid.Parse(raw)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid X-User-ID header")
				return
			}
		}

		ctx := context.WithValue(r.Context(), identityContextKey{}, requestIdentity{tenantID: tenantID, userID: userID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TenantIDFrom returns the tenant identity established for the request.
func TenantIDFrom(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctx.Value(identityContextKey{}).(requestIdentity)
	if !ok || id.tenantID == uuid.Nil {
		return uuid.Nil, errors.New("no tenant identity in request context")
	}
	return id.tenantID, nil
}

// UserIDFrom returns the user identity established for the request.
func UserIDFrom(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctx.Value(identityContextKey{}).(requestIdentity)
	if !ok || id.userID == uuid.Nil {
		return uuid.Nil, errors.New("no user identity in request context")
	}
	return id.userID, nil
}

func parseUUIDHeader(r *http.Request, name string) (uuid.UUID, error) {
	raw := r.Header.Get(name)
	if raw == "" {
		return uuid.Nil, errors.New("header missing")
	}
	return uuid.Parse(raw)
}
