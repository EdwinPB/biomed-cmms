// Package httpapi wires HTTP routes to application services using only the
// standard library net/http package.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

// TenantService is the application use-case boundary consumed by the HTTP
// layer. Implementations are the tenant service; the HTTP layer never touches
// the repository or PostgreSQL directly.
type TenantService interface {
	CreateTenant(ctx context.Context, params tenant.CreateParams) (tenant.Tenant, error)
}

type handler struct {
	tenants TenantService
}

// NewHandler builds the HTTP handler with all routes registered.
func NewHandler(tenants TenantService) http.Handler {
	h := &handler{tenants: tenants}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /api/v1/tenants", h.createTenant)
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
