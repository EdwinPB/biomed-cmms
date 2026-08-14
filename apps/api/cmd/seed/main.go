// Command seed inserts the idempotent demo dataset into the configured
// database.
//
// Usage:
//
//	go run ./cmd/seed
//
// Passwords default to seed.DemoPassword; override with SEED_DEMO_PASSWORD.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/edwinpolo/biomed-cmms/api/internal/config"
	"github.com/edwinpolo/biomed-cmms/api/internal/database"
	"github.com/edwinpolo/biomed-cmms/api/internal/seed"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer pool.Close()

	password := os.Getenv("SEED_DEMO_PASSWORD")
	if password == "" {
		password = seed.DemoPassword
	}

	s, err := seed.Run(ctx, pool, password)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}

	fmt.Printf("seed complete: tenants=%d users=%d equipment=%d requests=%d events=%d rfps=%d\n",
		s.Tenants, s.Users, s.Equipment, s.Requests, s.Events, s.RFPs)
}
