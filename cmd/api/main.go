package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erp/pkg/config"
	"erp/pkg/httpserver"
	biclient "erp/services/reports-service/internal/adapters/bi"
	configclient "erp/services/reports-service/internal/adapters/config"
	httpadapter "erp/services/reports-service/internal/adapters/http"
	purchasingclient "erp/services/reports-service/internal/adapters/purchasing"
	salesclient "erp/services/reports-service/internal/adapters/sales"
	stockclient "erp/services/reports-service/internal/adapters/stock"
	"erp/services/reports-service/internal/application"
)

// reports-service holds no database of its own: every report is built by
// calling the owning service's existing HTTP API on demand. That keeps it
// from ever sharing a connection pool, or contending for locks, with the
// transactional services it reports on.
func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	svc := application.New(
		stockclient.New(cfg.StockBaseURL),
		salesclient.New(cfg.SalesBaseURL),
		purchasingclient.New(cfg.PurchasingBaseURL),
		configclient.New(cfg.ConfigBaseURL),
		biclient.New(cfg.BiBaseURL),
	)

	engine := httpserver.New(cfg.ServiceName)
	httpadapter.New(svc).Register(engine, httpserver.JWT(cfg.JWTSecret, cfg.JWTIssuer))

	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: engine}
	go func() {
		log.Printf("%s listening on %s", cfg.ServiceName, srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
