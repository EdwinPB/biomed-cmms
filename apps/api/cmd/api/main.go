// Command api runs the biomed-cmms HTTP API.
//
// Wiring (all in this entrypoint):
//
//	config → database → postgres repository → tenant service → http handler
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edwinpolo/biomed-cmms/api/internal/config"
	"github.com/edwinpolo/biomed-cmms/api/internal/database"
	"github.com/edwinpolo/biomed-cmms/api/internal/httpapi"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant/postgres"
	"github.com/edwinpolo/biomed-cmms/api/internal/tenant/service"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	repo := postgres.NewRepository(pool)
	tenants := service.New(repo)

	server := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           httpapi.NewHandler(tenants),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("api listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("api server: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("api shutdown: %v", err)
	}
}
