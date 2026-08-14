// Package seed inserts an idempotent demo dataset for local and demo
// environments. It is never invoked automatically; run it explicitly with
// `go run ./cmd/seed`.
//
// Idempotency strategy: every row has a deterministic UUID, and every INSERT
// uses ON CONFLICT DO NOTHING keyed on the natural unique keys (tenant slug,
// tenant+email, tenant+asset_tag) or the fixed primary key. Running the seed
// repeatedly never produces duplicates and fills in any missing rows.
package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	// DemoTenantSlug is the slug of the seeded demo tenant.
	DemoTenantSlug = "demo"
	// DemoPassword is the default password for every demo user.
	DemoPassword = "DemoPass!123"
)

// Summary reports row counts after seeding.
type Summary struct {
	Tenants   int
	Users     int
	Equipment int
	Requests  int
	Events    int
	RFPs      int
}

var (
	tenantID = uuid.MustParse("10000000-0000-0000-0000-000000000001")

	adminID     = uuid.MustParse("10000000-0000-0000-0000-000000000101")
	requesterID = uuid.MustParse("10000000-0000-0000-0000-000000000102")
	biomedicID  = uuid.MustParse("10000000-0000-0000-0000-000000000103")

	equipmentIDs = []uuid.UUID{
		uuid.MustParse("20000000-0000-0000-0000-000000000001"),
		uuid.MustParse("20000000-0000-0000-0000-000000000002"),
		uuid.MustParse("20000000-0000-0000-0000-000000000003"),
		uuid.MustParse("20000000-0000-0000-0000-000000000004"),
		uuid.MustParse("20000000-0000-0000-0000-000000000005"),
		uuid.MustParse("20000000-0000-0000-0000-000000000006"),
		uuid.MustParse("20000000-0000-0000-0000-000000000007"),
		uuid.MustParse("20000000-0000-0000-0000-000000000008"),
		uuid.MustParse("20000000-0000-0000-0000-000000000009"),
		uuid.MustParse("20000000-0000-0000-0000-000000000010"),
	}

	requestIDs = []uuid.UUID{
		uuid.MustParse("30000000-0000-0000-0000-000000000001"),
		uuid.MustParse("30000000-0000-0000-0000-000000000002"),
		uuid.MustParse("30000000-0000-0000-0000-000000000003"),
		uuid.MustParse("30000000-0000-0000-0000-000000000004"),
		uuid.MustParse("30000000-0000-0000-0000-000000000005"),
		uuid.MustParse("30000000-0000-0000-0000-000000000006"),
		uuid.MustParse("30000000-0000-0000-0000-000000000007"),
	}

	rfpIDs = []uuid.UUID{
		uuid.MustParse("50000000-0000-0000-0000-000000000001"),
		uuid.MustParse("50000000-0000-0000-0000-000000000002"),
	}
)

type equipmentRow struct {
	ID           uuid.UUID
	AssetTag     string
	Name         string
	SerialNumber string
	Location     string
	Status       string
}

type requestRow struct {
	ID          uuid.UUID
	Equipment   uuid.UUID
	Title       string
	Description string
	Priority    string
	Status      string
	CreatedBy   uuid.UUID
	AssignedTo  *uuid.UUID
	Resolution  *string
}

type eventRow struct {
	ID         uuid.UUID
	Request    uuid.UUID
	Actor      uuid.UUID
	FromStatus string
	ToStatus   string
}

type rfpRow struct {
	ID               uuid.UUID
	ServiceRequestID uuid.UUID
	Title            string
	Description      string
	Status           string
	DueAt            *time.Time
	CreatedBy        uuid.UUID
}

func strPtr(s string) *string { return &s }

var equipment = []equipmentRow{
	{equipmentIDs[0], "EQ-0001", "Infusion Pump", "SN-IP-1001", "ICU-Room-1", "operational"},
	{equipmentIDs[1], "EQ-0002", "Ventilator", "SN-VT-2001", "ICU-Room-2", "operational"},
	{equipmentIDs[2], "EQ-0003", "ECG Monitor", "SN-EC-3001", "ER-Bay-1", "operational"},
	{equipmentIDs[3], "EQ-0004", "Defibrillator", "SN-DF-4001", "ER-Bay-2", "operational"},
	{equipmentIDs[4], "EQ-0005", "Patient Monitor", "SN-PM-5001", "ICU-Room-3", "operational"},
	{equipmentIDs[5], "EQ-0006", "Ultrasound", "SN-US-6001", "Imaging-1", "operational"},
	{equipmentIDs[6], "EQ-0007", "Anesthesia Machine", "SN-AN-7001", "OR-1", "operational"},
	{equipmentIDs[7], "EQ-0008", "X-Ray Unit", "SN-XR-8001", "Imaging-2", "operational"},
	{equipmentIDs[8], "EQ-0009", "Centrifuge", "SN-CF-9001", "Lab-1", "maintenance"},
	{equipmentIDs[9], "EQ-0010", "Autoclave", "SN-AU-10001", "Sterilization", "maintenance"},
}

var requests = []requestRow{
	{requestIDs[0], equipmentIDs[0], "Infusion pump not starting",
		"Unit powers on but errors at startup", "high", "pending", requesterID, nil, nil},
	{requestIDs[1], equipmentIDs[1], "Ventilator alarm intermittent",
		"Alarm fires intermittently on low pressure circuit", "critical", "pending", requesterID, nil, nil},
	{requestIDs[2], equipmentIDs[2], "ECG leads faulty",
		"Leads report noisy signal after a few minutes", "medium", "assigned", requesterID, &biomedicID, nil},
	{requestIDs[3], equipmentIDs[3], "Defibrillator battery draining",
		"Battery drops charge overnight", "high", "in_progress", requesterID, &biomedicID, nil},
	{requestIDs[4], equipmentIDs[5], "Ultrasound image flicker",
		"Display flickers during probe use", "medium", "in_progress", requesterID, &biomedicID, nil},
	{requestIDs[5], equipmentIDs[6], "Anesthesia gas leak check",
		"Suspect seal leak on breathing circuit", "high", "resolved", requesterID, &biomedicID,
		strPtr("Replaced circuit seal; passed leak test")},
	{requestIDs[6], equipmentIDs[4], "Patient monitor probe",
		"SpO2 probe unreliable", "low", "cancelled", requesterID, nil, nil},
}

// Event chains follow the legal transitions defined in
// internal/servicerequest: pending->assigned|cancelled,
// assigned->in_progress|cancelled, in_progress->resolved|cancelled.
var events = []eventRow{
	{eventID(1), requestIDs[2], adminID, "pending", "assigned"},
	{eventID(2), requestIDs[3], adminID, "pending", "assigned"},
	{eventID(3), requestIDs[3], biomedicID, "assigned", "in_progress"},
	{eventID(4), requestIDs[4], adminID, "pending", "assigned"},
	{eventID(5), requestIDs[4], biomedicID, "assigned", "in_progress"},
	{eventID(6), requestIDs[5], adminID, "pending", "assigned"},
	{eventID(7), requestIDs[5], biomedicID, "assigned", "in_progress"},
	{eventID(8), requestIDs[5], biomedicID, "in_progress", "resolved"},
	{eventID(9), requestIDs[6], requesterID, "pending", "cancelled"},
}

func eventID(n int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("40000000-0000-0000-0000-%012d", n))
}

var rfps = []rfpRow{
	{rfpIDs[0], requestIDs[0], "Infusion pump service RFP",
		"Requesting proposals for out-of-warranty pump servicing", "draft",
		ptr(time.Now().AddDate(0, 0, 14)), adminID},
	{rfpIDs[1], requestIDs[1], "Ventilator maintenance RFP",
		"Requesting proposals for preventive maintenance on ventilators", "published",
		ptr(time.Now().AddDate(0, 0, 7)), adminID},
}

func ptr(t time.Time) *time.Time { return &t }

// Run inserts the demo dataset inside a single transaction. Every insert is
// idempotent (ON CONFLICT DO NOTHING). Passwords are hashed with bcrypt at
// DefaultCost; no password material is embedded in SQL.
func Run(ctx context.Context, pool *pgxpool.Pool, password string) (Summary, error) {
	var s Summary

	if password == "" {
		return s, fmt.Errorf("seed: password must not be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return s, fmt.Errorf("seed: hash password: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return s, fmt.Errorf("seed: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := []string{
		`INSERT INTO tenants (id, slug, name, status) VALUES ($1, $2, $3, 'active')
		 ON CONFLICT (slug) DO NOTHING`,
		`INSERT INTO users (id, tenant_id, email, password_hash, full_name, role, is_active) VALUES ($1, $2, $3, $4, $5, $6, true)
		 ON CONFLICT (tenant_id, email) DO NOTHING`,
		`INSERT INTO equipment (id, tenant_id, asset_tag, name, serial_number, location, status) VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (tenant_id, asset_tag) DO NOTHING`,
		`INSERT INTO service_requests (id, tenant_id, equipment_id, title, description, priority, status, created_by, assigned_to, resolution_notes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO request_events (id, tenant_id, request_id, actor_id, from_status, to_status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO NOTHING`,
		`INSERT INTO rfps (id, tenant_id, service_request_id, title, description, status, due_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id) DO NOTHING`,
	}

	exec := func(sql string, args ...any) error {
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return err
		}
		return nil
	}

	if err := exec(queries[0], tenantID, DemoTenantSlug, "Demo Hospital"); err != nil {
		return s, fmt.Errorf("seed: tenant: %w", err)
	}

	users := []struct {
		id    uuid.UUID
		email string
		name  string
		role  string
	}{
		{adminID, "admin@demo.test", "Admin User", "admin"},
		{requesterID, "requester@demo.test", "Requester User", "requester"},
		{biomedicID, "biomedic@demo.test", "Biomedic User", "biomedic"},
	}
	for _, u := range users {
		if err := exec(queries[1], u.id, tenantID, u.email, string(hash), u.name, u.role); err != nil {
			return s, fmt.Errorf("seed: user %s: %w", u.email, err)
		}
	}

	for _, e := range equipment {
		if err := exec(queries[2], e.ID, tenantID, e.AssetTag, e.Name, e.SerialNumber, e.Location, e.Status); err != nil {
			return s, fmt.Errorf("seed: equipment %s: %w", e.AssetTag, err)
		}
	}

	for _, r := range requests {
		if err := exec(queries[3], r.ID, tenantID, r.Equipment, r.Title, r.Description, r.Priority, r.Status, r.CreatedBy, r.AssignedTo, r.Resolution); err != nil {
			return s, fmt.Errorf("seed: request %s: %w", r.Title, err)
		}
	}

	for _, ev := range events {
		if err := exec(queries[4], ev.ID, tenantID, ev.Request, ev.Actor, ev.FromStatus, ev.ToStatus); err != nil {
			return s, fmt.Errorf("seed: event for request %s: %w", ev.Request, err)
		}
	}

	for _, r := range rfps {
		if err := exec(queries[5], r.ID, tenantID, r.ServiceRequestID, r.Title, r.Description, r.Status, r.DueAt, r.CreatedBy); err != nil {
			return s, fmt.Errorf("seed: rfp %s: %w", r.Title, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return s, fmt.Errorf("seed: commit: %w", err)
	}

	counts := []struct {
		table string
		dest  *int
	}{
		{"tenants", &s.Tenants},
		{"users", &s.Users},
		{"equipment", &s.Equipment},
		{"service_requests", &s.Requests},
		{"request_events", &s.Events},
		{"rfps", &s.RFPs},
	}
	for _, c := range counts {
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM `+c.table).Scan(c.dest); err != nil {
			return s, fmt.Errorf("seed: count %s: %w", c.table, err)
		}
	}

	return s, nil
}
