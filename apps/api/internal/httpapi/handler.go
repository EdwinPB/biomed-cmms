// Package httpapi wires HTTP routes to application services using only the
// standard library net/http package.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/edwinpolo/biomed-cmms/api/internal/auth"
	"github.com/edwinpolo/biomed-cmms/api/internal/equipment"
	"github.com/edwinpolo/biomed-cmms/api/internal/rfp"
	"github.com/edwinpolo/biomed-cmms/api/internal/servicerequest"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant"
)

// HealthChecker verifies connectivity to the database. The pgx pool satisfies
// this interface.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// TenantService is the application use-case boundary consumed by the HTTP
// layer. Implementations are the tenant service; the HTTP layer never touches
// the repository or PostgreSQL directly.
type TenantService interface {
	CreateTenant(ctx context.Context, params tenant.CreateParams) (tenant.Tenant, error)
}

// AuthService is the authentication use-case boundary. Implementations are the
// auth service; the HTTP layer never touches the repository or PostgreSQL
// directly.
type AuthService interface {
	Login(ctx context.Context, creds auth.Credentials) (auth.Session, error)
	Logout(ctx context.Context, tokenHash string) error
	Authenticate(ctx context.Context, tokenHash string) (auth.Principal, error)
	ListUsers(ctx context.Context, tenantID uuid.UUID, role auth.Role) ([]auth.User, error)
	CreateUser(ctx context.Context, params auth.CreateParams, role auth.Role) (auth.User, error)
	UpdateUser(ctx context.Context, params auth.UpdateParams, actorUserID uuid.UUID, role auth.Role) (auth.User, error)
}

// ServiceRequestService is the application use-case boundary for service
// requests. Implementations are the servicerequest service; the HTTP layer
// never touches the repository or PostgreSQL directly.
type ServiceRequestService interface {
	CreateRequest(ctx context.Context, params servicerequest.CreateParams) (servicerequest.ServiceRequest, error)
	TransitionRequest(ctx context.Context, tenantID, id, actorID uuid.UUID, role auth.Role, to servicerequest.Status) (servicerequest.ServiceRequest, error)
	RequestHistory(ctx context.Context, tenantID, requestID, userID uuid.UUID, role auth.Role) ([]servicerequest.RequestEvent, error)
	GetRequest(ctx context.Context, tenantID, id, userID uuid.UUID, role auth.Role) (servicerequest.ServiceRequest, error)
	ListRequests(ctx context.Context, tenantID, userID uuid.UUID, role auth.Role) ([]servicerequest.ServiceRequest, error)
}

// RFPService is the application use-case boundary for RFPs. Implementations are
// the rfp service; the HTTP layer never touches the repository or PostgreSQL
// directly.
type RFPService interface {
	CreateRFP(ctx context.Context, params rfp.CreateParams, role auth.Role) (rfp.RFP, error)
	TransitionRFP(ctx context.Context, tenantID, id uuid.UUID, role auth.Role, to rfp.Status) (rfp.RFP, error)
	GetRFP(ctx context.Context, tenantID, id uuid.UUID, role auth.Role) (rfp.RFP, error)
	GetRFPByServiceRequest(ctx context.Context, tenantID, serviceRequestID uuid.UUID, role auth.Role) (rfp.RFP, error)
}

// EquipmentService is the application use-case boundary for equipment.
// Implementations are the equipment service; the HTTP layer never touches the
// repository or PostgreSQL directly.
type EquipmentService interface {
	ListEquipment(ctx context.Context, tenantID uuid.UUID, role auth.Role) ([]equipment.Equipment, error)
	ListSelectable(ctx context.Context, tenantID uuid.UUID) ([]equipment.Equipment, error)
}

type handler struct {
	tenants           TenantService
	auth              AuthService
	serviceRequests   ServiceRequestService
	rfps              RFPService
	equipment         EquipmentService
	healthChecker     HealthChecker
	sessionCookieName string
}

// NewHandler builds the HTTP handler with all routes registered.
func NewHandler(tenants TenantService, authService AuthService, serviceRequests ServiceRequestService, rfps RFPService, equipment EquipmentService, health HealthChecker, sessionCookieName string) http.Handler {
	h := &handler{
		tenants:           tenants,
		auth:              authService,
		serviceRequests:   serviceRequests,
		rfps:              rfps,
		equipment:         equipment,
		healthChecker:     health,
		sessionCookieName: sessionCookieName,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.Handle("POST /api/v1/tenants", h.session(http.HandlerFunc(h.createTenant)))
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.Handle("POST /api/v1/auth/logout", h.session(http.HandlerFunc(h.logout)))
	mux.Handle("GET /api/v1/auth/me", h.session(http.HandlerFunc(h.me)))
	mux.Handle("GET /api/v1/users", h.session(http.HandlerFunc(h.listUsers)))
	mux.Handle("POST /api/v1/users", h.session(http.HandlerFunc(h.createUser)))
	mux.Handle("PATCH /api/v1/users/{id}", h.session(http.HandlerFunc(h.updateUser)))
	mux.Handle("POST /api/v1/requests", h.session(http.HandlerFunc(h.createRequest)))
	mux.Handle("GET /api/v1/requests", h.session(http.HandlerFunc(h.listRequests)))
	mux.Handle("GET /api/v1/requests/{id}", h.session(http.HandlerFunc(h.getRequest)))
	mux.Handle("PATCH /api/v1/requests/{id}/status", h.session(http.HandlerFunc(h.transitionStatus)))
	mux.Handle("GET /api/v1/requests/{id}/history", h.session(http.HandlerFunc(h.requestHistory)))
	mux.Handle("GET /api/v1/equipment", h.session(http.HandlerFunc(h.listEquipment)))
	mux.Handle("GET /api/v1/equipment/selectable", h.session(http.HandlerFunc(h.listSelectableEquipment)))
	mux.Handle("POST /api/v1/rfps", h.session(http.HandlerFunc(h.createRFP)))
	mux.Handle("PATCH /api/v1/rfps/{id}/status", h.session(http.HandlerFunc(h.transitionRFPStatus)))
	mux.Handle("GET /api/v1/rfps/{id}", h.session(http.HandlerFunc(h.getRFP)))
	mux.Handle("GET /api/v1/service-requests/{id}/rfp", h.session(http.HandlerFunc(h.getRFPByServiceRequest)))
	return mux
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.healthChecker.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

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
