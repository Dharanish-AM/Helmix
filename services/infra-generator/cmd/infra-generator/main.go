package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/your-org/helmix/services/infra-generator/internal/config"
	"github.com/your-org/helmix/services/infra-generator/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	infraServer := server.New(logger)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           infraServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("infra-generator listening", slog.String("addr", httpServer.Addr))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server failed: " + err.Error())
	}
}
