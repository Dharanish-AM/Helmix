package main

import (
	"database/sql"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/your-org/helmix/services/deployment-engine/internal/config"
	"github.com/your-org/helmix/services/deployment-engine/internal/deploy"
	"github.com/your-org/helmix/services/deployment-engine/internal/server"
	"github.com/your-org/helmix/services/deployment-engine/internal/store"
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

	deploymentStore := store.New(database)
	publisher, err := deploy.NewPublisher(cfg.NATSURL)
	if err != nil {
		log.Fatal("connect publisher failed: " + err.Error())
	}
	defer func() { _ = publisher.Close() }()

	engine := deploy.NewService(logger, deploymentStore, publisher, cfg.DeployTimeout, cfg.DefaultReadyDelay)
	deploymentServer := server.New(logger, engine)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           deploymentServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("deployment-engine listening", slog.String("addr", httpServer.Addr))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server failed: " + err.Error())
	}
}
