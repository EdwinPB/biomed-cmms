package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
)

type loginRequest struct {
	TenantSlug string `json:"tenant_slug"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

type authUserResponse struct {
	ID       uuid.UUID `json:"id"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	Role     auth.Role `json:"role"`
}

type authTenantResponse struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

type authResponse struct {
	User   authUserResponse   `json:"user"`
	Tenant authTenantResponse `json:"tenant"`
}

// login validates credentials and issues a session cookie.
func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.TenantSlug) == "" || strings.TrimSpace(req.Email) == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "tenant_slug, email and password are required")
		return
	}

	session, err := h.auth.Login(r.Context(), auth.Credentials{
		TenantSlug: req.TenantSlug,
		Email:      req.Email,
		Password:   req.Password,
	})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) || errors.Is(err, auth.ErrUserInactive) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http.SetCookie(w, h.sessionCookie(session.Token, session.ExpiresAt, false))
	writeJSON(w, http.StatusOK, authResponse{
		User: authUserResponse{
			ID:       session.User.ID,
			Email:    session.User.Email,
			FullName: session.User.FullName,
			Role:     session.User.Role,
		},
		Tenant: authTenantResponse{
			ID:   session.Tenant.ID,
			Slug: session.Tenant.Slug,
			Name: session.Tenant.Name,
		},
	})
}

// logout revokes the current session and clears the session cookie.
func (h *handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(h.sessionCookieName); err == nil {
		_ = h.auth.Logout(r.Context(), auth.HashToken(c.Value))
	}

	http.SetCookie(w, h.sessionCookie("", time.Time{}, true))
	w.WriteHeader(http.StatusNoContent)
}

// me returns the authenticated user and their tenant.
func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	p, err := principalFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		User: authUserResponse{
			ID:       p.UserID,
			Email:    p.Email,
			FullName: p.FullName,
			Role:     p.Role,
		},
		Tenant: authTenantResponse{
			ID:   p.TenantID,
			Slug: p.TenantSlug,
			Name: p.TenantName,
		},
	})
}

// sessionCookie builds the cookie that carries the session token. When clear is
// true it produces a cookie that instructs the browser to delete it.
func (h *handler) sessionCookie(token string, expires time.Time, clear bool) *http.Cookie {
	c := &http.Cookie{
		Name:     h.sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}
	if clear {
		c.Value = ""
		c.MaxAge = -1
		c.Expires = time.Unix(1, 0)
		return c
	}
	c.MaxAge = int(time.Until(expires).Seconds())
	c.Expires = expires
	return c
}
