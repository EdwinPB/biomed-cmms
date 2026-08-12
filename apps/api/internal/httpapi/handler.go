// Package httpapi wires HTTP routes to application services using only the
// standard library net/http package.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

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
	TransitionRequest(ctx context.Context, tenantID, id uuid.UUID, to servicerequest.Status) (servicerequest.ServiceRequest, error)
}

type handler struct {
	tenants         TenantService
	serviceRequests ServiceRequestService
}

// NewHandler builds the HTTP handler with all routes registered.
func NewHandler(tenants TenantService, serviceRequests ServiceRequestService) http.Handler {
	h := &handler{tenants: tenants, serviceRequests: serviceRequests}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /api/v1/tenants", h.createTenant)
	mux.Handle("POST /api/v1/requests", h.identity(http.HandlerFunc(h.createRequest)))
	mux.Handle("PATCH /api/v1/requests/{id}/status", h.identity(http.HandlerFunc(h.transitionStatus)))
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
