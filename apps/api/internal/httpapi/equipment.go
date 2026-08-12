package httpapi

import (
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

func (h *handler) listEquipment(w http.ResponseWriter, r *http.Request) {
	tenantID, err := TenantIDFrom(r.Context())
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	items, err := h.equipment.ListEquipment(r.Context(), tenantID)
	if err != nil {
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
