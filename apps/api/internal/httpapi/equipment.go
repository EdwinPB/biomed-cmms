package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
)

type equipmentResponse struct {
	ID           uuid.UUID        `json:"id"`
	AssetTag     string           `json:"asset_tag"`
	Name         string           `json:"name"`
	SerialNumber string           `json:"serial_number"`
	Location     string           `json:"location"`
	Status       equipment.Status `json:"status"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
}

type equipmentListResponse struct {
	Equipment []equipmentResponse `json:"equipment"`
}

// selectableEquipmentResponse is the bounded view exposed to requesters for
// request-creation selection. It deliberately omits serial_number and
// timestamps, which remain staff-only via equipmentResponse.
type selectableEquipmentResponse struct {
	ID       uuid.UUID        `json:"id"`
	AssetTag string           `json:"asset_tag"`
	Name     string           `json:"name"`
	Location string           `json:"location"`
	Status   equipment.Status `json:"status"`
}

type selectableEquipmentListResponse struct {
	Equipment []selectableEquipmentResponse `json:"equipment"`
}

func (h *handler) listEquipment(w http.ResponseWriter, r *http.Request) {
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

	items, err := h.equipment.ListEquipment(r.Context(), tenantID, role)
	if err != nil {
		if errors.Is(err, equipment.ErrForbidden) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]equipmentResponse, 0, len(items))
	for _, e := range items {
		resp = append(resp, equipmentResponse{
			ID:           e.ID,
			AssetTag:     e.AssetTag,
			Name:         e.Name,
			SerialNumber: e.SerialNumber,
			Location:     e.Location,
			Status:       e.Status,
			CreatedAt:    e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:    e.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, http.StatusOK, equipmentListResponse{Equipment: resp})
}

// listSelectableEquipment returns the tenant's equipment as a bounded
// selection view for request creation. It is available to every authenticated
// role, including requesters, but never includes serial_number or timestamps.
func (h *handler) listSelectableEquipment(w http.ResponseWriter, r *http.Request) {
	tenantID, err := TenantIDFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	items, err := h.equipment.ListSelectable(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]selectableEquipmentResponse, 0, len(items))
	for _, e := range items {
		resp = append(resp, selectableEquipmentResponse{
			ID:       e.ID,
			AssetTag: e.AssetTag,
			Name:     e.Name,
			Location: e.Location,
			Status:   e.Status,
		})
	}
	writeJSON(w, http.StatusOK, selectableEquipmentListResponse{Equipment: resp})
}
