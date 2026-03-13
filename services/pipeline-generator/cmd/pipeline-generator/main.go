package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/your-org/helmix/services/pipeline-generator/internal/config"
	"github.com/your-org/helmix/services/pipeline-generator/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	pipelineServer := server.New(logger)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           pipelineServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("pipeline-generator listening", slog.String("addr", httpServer.Addr))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server failed: " + err.Error())
	}
}
