package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/your-org/helmix/services/repo-analyzer/internal/config"
	"github.com/your-org/helmix/services/repo-analyzer/internal/server"
	"github.com/your-org/helmix/services/repo-analyzer/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	config, err := config.Load()
	if err != nil {
		logger.Error("load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	databaseStore, err := store.Open(config.DatabaseURL)
	if err != nil {
		logger.Error("open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer databaseStore.Close()

	natsClient, err := nats.Connect(config.NATSURL)
	if err != nil {
		logger.Error("connect nats", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer natsClient.Close()

	handler := server.New(config, logger, databaseStore, natsClient)
	httpServer := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("repo-analyzer listening", slog.String("addr", httpServer.Addr))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server failed: " + err.Error())
	}
}
