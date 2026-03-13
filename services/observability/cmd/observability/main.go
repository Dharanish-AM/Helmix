package main

import (
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/your-org/helmix/services/observability/internal/config"
	"github.com/your-org/helmix/services/observability/internal/observability"
	"github.com/your-org/helmix/services/observability/internal/server"
	"github.com/your-org/helmix/services/observability/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("load config failed: " + err.Error())
	}

	database, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal("open database failed: " + err.Error())
	}
	defer database.Close()

	if err := database.Ping(); err != nil {
		log.Fatal("ping database failed: " + err.Error())
	}

	observabilityStore := store.New(database)
	publisher, err := observability.NewPublisher(cfg.NATSURL)
	if err != nil {
		log.Fatal("connect publisher failed: " + err.Error())
	}
	defer func() { _ = publisher.Close() }()

	service := observability.NewService(logger, observabilityStore, publisher)
	applicationServer := server.New(logger, service)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           applicationServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	metricsServer := &http.Server{
		Addr:              ":" + cfg.MetricsPort,
		Handler:           promhttp.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("observability metrics listening", slog.String("addr", metricsServer.Addr))
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("metrics server failed: " + err.Error())
		}
	}()

	logger.Info("observability listening", slog.String("addr", httpServer.Addr))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server failed: " + err.Error())
	}
}
