package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant/service"
)

type createTenantRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type tenantResponse struct {
	ID        uuid.UUID     `json:"id"`
	Slug      string        `json:"slug"`
	Name      string        `json:"name"`
	Status    tenant.Status `json:"status"`
	CreatedAt string        `json:"created_at"`
	UpdatedAt string        `json:"updated_at"`
}

func (h *handler) createTenant(w http.ResponseWriter, r *http.Request) {
	var req createTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	created, err := h.tenants.CreateTenant(r.Context(), tenant.CreateParams{Slug: req.Slug, Name: req.Name})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSlugRequired), errors.Is(err, service.ErrNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, tenant.ErrConflict):
			writeError(w, http.StatusConflict, "tenant already exists")
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, tenantResponse{
		ID:        created.ID,
		Slug:      created.Slug,
		Name:      created.Name,
		Status:    created.Status,
		CreatedAt: created.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: created.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}
