// Package httpapi wires HTTP routes to application services using only the
// standard library net/http package.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
	"github.com/edwinpolo/biomed-cmms/api/internal/rfp"
	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

// TenantService is the application use-case boundary consumed by the HTTP
// layer. Implementations are the tenant service; the HTTP layer never touches
// the repository or PostgreSQL directly.
type TenantService interface {
	CreateTenant(ctx context.Context, params tenant.CreateParams) (tenant.Tenant, error)
}

// ServiceRequestService is the application use-case boundary for service
// requests. Implementations are the servicerequest service; the HTTP layer
// never touches the repository or PostgreSQL directly.
type ServiceRequestService interface {
	CreateRequest(ctx context.Context, params servicerequest.CreateParams) (servicerequest.ServiceRequest, error)
	TransitionRequest(ctx context.Context, tenantID, id, actorID uuid.UUID, to servicerequest.Status) (servicerequest.ServiceRequest, error)
	RequestHistory(ctx context.Context, tenantID, requestID uuid.UUID) ([]servicerequest.RequestEvent, error)
	GetRequest(ctx context.Context, tenantID, id uuid.UUID) (servicerequest.ServiceRequest, error)
	ListRequests(ctx context.Context, tenantID uuid.UUID) ([]servicerequest.ServiceRequest, error)
}

// RFPService is the application use-case boundary for RFPs. Implementations are
// the rfp service; the HTTP layer never touches the repository or PostgreSQL
// directly.
type RFPService interface {
	CreateRFP(ctx context.Context, params rfp.CreateParams) (rfp.RFP, error)
	TransitionRFP(ctx context.Context, tenantID, id uuid.UUID, to rfp.Status) (rfp.RFP, error)
	GetRFP(ctx context.Context, tenantID, id uuid.UUID) (rfp.RFP, error)
	GetRFPByServiceRequest(ctx context.Context, tenantID, serviceRequestID uuid.UUID) (rfp.RFP, error)
}

// EquipmentService is the application use-case boundary for equipment.
// Implementations are the equipment service; the HTTP layer never touches the
// repository or PostgreSQL directly.
type EquipmentService interface {
	ListEquipment(ctx context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error)
}

type handler struct {
	tenants         TenantService
	serviceRequests ServiceRequestService
	rfps            RFPService
	equipment       EquipmentService
}

// NewHandler builds the HTTP handler with all routes registered.
func NewHandler(tenants TenantService, serviceRequests ServiceRequestService, rfps RFPService, equipment EquipmentService) http.Handler {
	h := &handler{tenants: tenants, serviceRequests: serviceRequests, rfps: rfps, equipment: equipment}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /api/v1/tenants", h.createTenant)
	mux.Handle("POST /api/v1/requests", h.identity(http.HandlerFunc(h.createRequest)))
	mux.Handle("GET /api/v1/requests", h.identity(http.HandlerFunc(h.listRequests)))
	mux.Handle("GET /api/v1/requests/{id}", h.identity(http.HandlerFunc(h.getRequest)))
	mux.Handle("PATCH /api/v1/requests/{id}/status", h.identity(http.HandlerFunc(h.transitionStatus)))
	mux.Handle("GET /api/v1/requests/{id}/history", h.identity(http.HandlerFunc(h.requestHistory)))
	mux.Handle("GET /api/v1/equipment", h.identity(http.HandlerFunc(h.listEquipment)))
	mux.Handle("POST /api/v1/rfps", h.identity(http.HandlerFunc(h.createRFP)))
	mux.Handle("PATCH /api/v1/rfps/{id}/status", h.identity(http.HandlerFunc(h.transitionRFPStatus)))
	mux.Handle("GET /api/v1/rfps/{id}", h.identity(http.HandlerFunc(h.getRFP)))
	mux.Handle("GET /api/v1/service-requests/{id}/rfp", h.identity(http.HandlerFunc(h.getRFPByServiceRequest)))
	return mux
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
