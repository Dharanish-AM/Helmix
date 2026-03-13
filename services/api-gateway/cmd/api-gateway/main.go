package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/your-org/helmix/services/api-gateway/internal/config"
	"github.com/your-org/helmix/services/api-gateway/internal/gateway"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	config, err := config.Load()
	if err != nil {
		logger.Error("load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	apiGateway, err := gateway.New(config, logger)
	if err != nil {
		logger.Error("build gateway", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer apiGateway.Close()

	httpServer := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           apiGateway.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("api-gateway listening", slog.String("addr", httpServer.Addr))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server failed: " + err.Error())
	}
}
