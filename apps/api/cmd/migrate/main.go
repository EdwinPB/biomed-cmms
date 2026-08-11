// Command migrate applies embedded SQL migrations to the application database.
//
// Usage:
//
//	go run ./cmd/migrate up      # apply all pending migrations
//	go run ./cmd/migrate down    # roll back one migration
//	go run ./cmd/migrate status  # show current migration version
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/edwinpolo/biomed-cmms/api/internal/config"
	"github.com/edwinpolo/biomed-cmms/api/internal/migrations"
)

func main() {
	cfg := config.Load()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		log.Fatalf("load embedded migrations: %v", err)
	}

	driver, err := pgxv5.WithInstance(db, &pgxv5.Config{})
	if err != nil {
		log.Fatalf("init migration driver: %v", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		log.Fatalf("init migrator: %v", err)
	}
	defer m.Close()

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "up":
		if err := m.Up(); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				fmt.Println("no pending migrations")
				return
			}
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("migrations applied")
	case "down":
		if err := m.Steps(-1); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				fmt.Println("at base version")
				return
			}
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Println("migration rolled back")
	case "status":
		version, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("no migrations applied")
			return
		}
		if err != nil {
			log.Fatalf("migrate status: %v", err)
		}
		fmt.Printf("version=%d dirty=%v\n", version, dirty)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (want up, down, or status)\n", cmd)
		os.Exit(2)
	}
}
