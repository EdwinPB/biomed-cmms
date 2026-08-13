package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/rfp"
	rfpservice "github.com/edwinpolo/biomed-cmms/api/internal/rfp/service"
)

type createRFPRequest struct {
	ServiceRequestID uuid.UUID  `json:"service_request_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	DueAt            *time.Time `json:"due_at"`
}

type transitionRFPRequest struct {
	Status rfp.Status `json:"status"`
}

type rfpResponse struct {
	ID               uuid.UUID  `json:"id"`
	ServiceRequestID uuid.UUID  `json:"service_request_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	Status           rfp.Status `json:"status"`
	DueAt            *string    `json:"due_at"`
	CreatedBy        uuid.UUID  `json:"created_by"`
	CreatedAt        string     `json:"created_at"`
	UpdatedAt        string     `json:"updated_at"`
}

func (h *handler) createRFP(w http.ResponseWriter, r *http.Request) {
	tenantID, err := TenantIDFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	userID, err := UserIDFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	role, err := RoleFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req createRFPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	created, err := h.rfps.CreateRFP(r.Context(), rfp.CreateParams{
		TenantID:         tenantID,
		ServiceRequestID: req.ServiceRequestID,
		Title:            req.Title,
		Description:      req.Description,
		DueAt:            req.DueAt,
		CreatedBy:        userID,
	}, role)
	if err != nil {
		h.writeRFPError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toRFPResponse(created))
}

func (h *handler) transitionRFPStatus(w http.ResponseWriter, r *http.Request) {
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

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rfp id")
		return
	}

	var req transitionRFPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	updated, err := h.rfps.TransitionRFP(r.Context(), tenantID, id, role, req.Status)
	if err != nil {
		h.writeRFPError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toRFPResponse(updated))
}

func (h *handler) getRFP(w http.ResponseWriter, r *http.Request) {
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

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rfp id")
		return
	}

	created, err := h.rfps.GetRFP(r.Context(), tenantID, id, role)
	if err != nil {
		h.writeRFPError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toRFPResponse(created))
}

func (h *handler) getRFPByServiceRequest(w http.ResponseWriter, r *http.Request) {
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

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid service request id")
		return
	}

	created, err := h.rfps.GetRFPByServiceRequest(r.Context(), tenantID, id, role)
	if err != nil {
		h.writeRFPError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toRFPResponse(created))
}

func (h *handler) writeRFPError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rfp.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, rfp.ErrNotFound):
		writeError(w, http.StatusNotFound, "rfp not found")
	case errors.Is(err, rfp.ErrInvalidTransition), errors.Is(err, rfp.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case isRFPValidationError(err):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func isRFPValidationError(err error) bool {
	return errors.Is(err, rfpservice.ErrTenantRequired) ||
		errors.Is(err, rfpservice.ErrServiceRequestRequired) ||
		errors.Is(err, rfpservice.ErrCreatedByRequired) ||
		errors.Is(err, rfpservice.ErrTitleRequired) ||
		errors.Is(err, rfpservice.ErrDescriptionRequired) ||
		errors.Is(err, rfpservice.ErrInvalidStatus) ||
		errors.Is(err, rfp.ErrPublishTitleRequired) ||
		errors.Is(err, rfp.ErrPublishDescriptionRequired) ||
		errors.Is(err, rfp.ErrPublishDueAtRequired) ||
		errors.Is(err, rfp.ErrPublishDueAtInPast)
}

func toRFPResponse(created rfp.RFP) rfpResponse {
	var dueAt *string
	if created.DueAt != nil {
		s := created.DueAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		dueAt = &s
	}
	return rfpResponse{
		ID:               created.ID,
		ServiceRequestID: created.ServiceRequestID,
		Title:            created.Title,
		Description:      created.Description,
		Status:           created.Status,
		DueAt:            dueAt,
		CreatedBy:        created.CreatedBy,
		CreatedAt:        created.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        created.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}
