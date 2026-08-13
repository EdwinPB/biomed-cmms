package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
)

// userResponse is the bounded view of a user exposed to admins. It deliberately
// omits password_hash, sessions, and token data.
type userResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      auth.Role `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt string    `json:"created_at"`
}

type userListResponse struct {
	Users []userResponse `json:"users"`
}

// createUserRequest carries the fields an admin may set. TenantID is never
// accepted here; it always comes from the authenticated session.
type createUserRequest struct {
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	Role     auth.Role `json:"role"`
	Password string    `json:"password"`
}

// listUsers returns the authenticated tenant's users. Admin-only: the service
// rejects biomedic and requester roles before any repository access.
func (h *handler) listUsers(w http.ResponseWriter, r *http.Request) {
	tenantID, err := TenantIDFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	role, err := RoleFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	users, err := h.auth.ListUsers(r.Context(), tenantID, role)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}
	writeJSON(w, http.StatusOK, userListResponse{Users: resp})
}

// createUser creates a user in the authenticated tenant. Admin-only; the
// service hashes the password and rejects duplicate tenant emails.
func (h *handler) createUser(w http.ResponseWriter, r *http.Request) {
	tenantID, err := TenantIDFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	role, err := RoleFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	created, err := h.auth.CreateUser(r.Context(), auth.CreateParams{
		TenantID:     tenantID,
		Email:        req.Email,
		FullName:     req.FullName,
		Role:         req.Role,
		PasswordHash: req.Password,
	}, role)
	if err != nil {
		h.writeUserError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toUserResponse(created))
}

func (h *handler) writeUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, auth.ErrConflict):
		writeError(w, http.StatusConflict, "user already exists")
	case isAuthValidationError(err):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func isAuthValidationError(err error) bool {
	return errors.Is(err, auth.ErrEmailRequired) ||
		errors.Is(err, auth.ErrPasswordRequired) ||
		errors.Is(err, auth.ErrInvalidRole)
}

func toUserResponse(u auth.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Email:     u.Email,
		FullName:  u.FullName,
		Role:      u.Role,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
	}
}
