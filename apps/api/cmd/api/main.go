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

	authpostgres "github.com/edwinpolo/biomed-cmms/api/internal/auth/postgres"
	authservice "github.com/edwinpolo/biomed-cmms/api/internal/auth/service"
	"github.com/edwinpolo/biomed-cmms/api/internal/config"
	"github.com/edwinpolo/biomed-cmms/api/internal/database"
	equipmentpostgres "github.com/edwinpolo/biomed-cmms/api/internal/equipment/postgres"
	equipmentservice "github.com/edwinpolo/biomed-cmms/api/internal/equipment/service"
	"github.com/edwinpolo/biomed-cmms/api/internal/httpapi"
	rfppostgres "github.com/edwinpolo/biomed-cmms/api/internal/rfp/postgres"
	rfpservice "github.com/edwinpolo/biomed-cmms/api/internal/rfp/service"
	srpostgres "github.com/edwinpolo/biomed-cmms/api/internal/servicerequest/postgres"
	srservice "github.com/edwinpolo/biomed-cmms/api/internal/servicerequest/service"
	tenantpostgres "github.com/edwinpolo/biomed-cmms/api/internal/tenant/postgres"
	tenantservice "github.com/edwinpolo/biomed-cmms/api/internal/tenant/service"
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

	repo := tenantpostgres.NewRepository(pool)
	tenants := tenantservice.New(repo)

	requestRepo := srpostgres.NewRepository(pool)
	serviceRequests := srservice.New(requestRepo)

	rfpRepo := rfppostgres.NewRepository(pool)
	rfps := rfpservice.New(rfpRepo)

	equipmentRepo := equipmentpostgres.NewRepository(pool)
	equipmentSvc := equipmentservice.New(equipmentRepo)

	authRepo := authpostgres.NewRepository(pool)
	authSvc := authservice.New(authRepo, cfg.SessionTTL)

	server := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           httpapi.CORS(cfg.CORSAllowedOrigin)(httpapi.NewHandler(tenants, authSvc, serviceRequests, rfps, equipmentSvc, cfg.SessionCookieName)),
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
