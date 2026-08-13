package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest"
	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest/service"
)

type createRequestRequest struct {
	EquipmentID uuid.UUID               `json:"equipment_id"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Priority    servicerequest.Priority `json:"priority"`
}

type transitionRequest struct {
	Status servicerequest.Status `json:"status"`
}

type requestResponse struct {
	ID              uuid.UUID               `json:"id"`
	TenantID        uuid.UUID               `json:"tenant_id"`
	EquipmentID     uuid.UUID               `json:"equipment_id"`
	Title           string                  `json:"title"`
	Description     string                  `json:"description"`
	Priority        servicerequest.Priority `json:"priority"`
	Status          servicerequest.Status   `json:"status"`
	CreatedBy       uuid.UUID               `json:"created_by"`
	AssignedTo      *uuid.UUID              `json:"assigned_to"`
	ResolutionNotes *string                 `json:"resolution_notes"`
	CreatedAt       string                  `json:"created_at"`
	UpdatedAt       string                  `json:"updated_at"`
}

func (h *handler) createRequest(w http.ResponseWriter, r *http.Request) {
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

	var req createRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	created, err := h.serviceRequests.CreateRequest(r.Context(), servicerequest.CreateParams{
		TenantID:    tenantID,
		EquipmentID: req.EquipmentID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		CreatedBy:   userID,
	})
	if err != nil {
		h.writeRequestError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toRequestResponse(created))
}

func (h *handler) transitionStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, err := TenantIDFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	actorID, err := UserIDFrom(r.Context())
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
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	var req transitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	updated, err := h.serviceRequests.TransitionRequest(r.Context(), tenantID, id, actorID, role, req.Status)
	if err != nil {
		h.writeRequestError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toRequestResponse(updated))
}

type requestEventResponse struct {
	ID         uuid.UUID             `json:"id"`
	ActorID    uuid.UUID             `json:"actor_id"`
	FromStatus servicerequest.Status `json:"from_status"`
	ToStatus   servicerequest.Status `json:"to_status"`
	CreatedAt  string                `json:"created_at"`
}

type requestHistoryResponse struct {
	Events []requestEventResponse `json:"events"`
}

type requestListResponse struct {
	Requests []requestResponse `json:"requests"`
}

func (h *handler) requestHistory(w http.ResponseWriter, r *http.Request) {
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

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	events, err := h.serviceRequests.RequestHistory(r.Context(), tenantID, id, userID, role)
	if err != nil {
		h.writeRequestError(w, err)
		return
	}

	resp := make([]requestEventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, requestEventResponse{
			ID:         e.ID,
			ActorID:    e.ActorID,
			FromStatus: e.FromStatus,
			ToStatus:   e.ToStatus,
			CreatedAt:  e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, http.StatusOK, requestHistoryResponse{Events: resp})
}

func (h *handler) getRequest(w http.ResponseWriter, r *http.Request) {
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

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request id")
		return
	}

	got, err := h.serviceRequests.GetRequest(r.Context(), tenantID, id, userID, role)
	if err != nil {
		h.writeRequestError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toRequestResponse(got))
}

func (h *handler) listRequests(w http.ResponseWriter, r *http.Request) {
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

	requests, err := h.serviceRequests.ListRequests(r.Context(), tenantID, userID, role)
	if err != nil {
		h.writeRequestError(w, err)
		return
	}

	resp := make([]requestResponse, 0, len(requests))
	for _, sr := range requests {
		resp = append(resp, toRequestResponse(sr))
	}
	writeJSON(w, http.StatusOK, requestListResponse{Requests: resp})
}

func (h *handler) writeRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, servicerequest.ErrNotFound):
		writeError(w, http.StatusNotFound, "service request not found")
	case errors.Is(err, servicerequest.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, servicerequest.ErrInvalidTransition):
		writeError(w, http.StatusConflict, err.Error())
	case isValidationError(err):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func isValidationError(err error) bool {
	return errors.Is(err, service.ErrTenantRequired) ||
		errors.Is(err, service.ErrEquipmentRequired) ||
		errors.Is(err, service.ErrCreatedByRequired) ||
		errors.Is(err, service.ErrTitleRequired) ||
		errors.Is(err, service.ErrDescriptionRequired) ||
		errors.Is(err, service.ErrInvalidPriority)
}

func toRequestResponse(sr servicerequest.ServiceRequest) requestResponse {
	return requestResponse{
		ID:              sr.ID,
		TenantID:        sr.TenantID,
		EquipmentID:     sr.EquipmentID,
		Title:           sr.Title,
		Description:     sr.Description,
		Priority:        sr.Priority,
		Status:          sr.Status,
		CreatedBy:       sr.CreatedBy,
		AssignedTo:      sr.AssignedTo,
		ResolutionNotes: sr.ResolutionNotes,
		CreatedAt:       sr.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       sr.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}
