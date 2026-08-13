package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
)

// Request identity is established from a cookie-backed server session by the
// session middleware and carried in the request context as an auth.Principal.
// The mechanism is isolated behind these helpers so business handlers never
// touch cookies or sessions directly.

type identityContextKey struct{}

// session reads the session cookie, resolves it through the auth service, and
// injects the authenticated principal into the request context. Missing,
// invalid, expired, or deactivated sessions return 401.
func (h *handler) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(h.sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		principal, err := h.auth.Authenticate(r.Context(), auth.HashToken(c.Value))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}

		ctx := context.WithValue(r.Context(), identityContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func principalFrom(ctx context.Context) (auth.Principal, error) {
	p, ok := ctx.Value(identityContextKey{}).(auth.Principal)
	if !ok || p.UserID == uuid.Nil {
		return auth.Principal{}, errors.New("no identity in request context")
	}
	return p, nil
}

// TenantIDFrom returns the tenant identity established for the request.
func TenantIDFrom(ctx context.Context) (uuid.UUID, error) {
	p, err := principalFrom(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return p.TenantID, nil
}

// UserIDFrom returns the user identity established for the request.
func UserIDFrom(ctx context.Context) (uuid.UUID, error) {
	p, err := principalFrom(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return p.UserID, nil
}

// RoleFrom returns the role of the authenticated user.
func RoleFrom(ctx context.Context) (auth.Role, error) {
	p, err := principalFrom(ctx)
	if err != nil {
		return "", err
	}
	return p.Role, nil
}
