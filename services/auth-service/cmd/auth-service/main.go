package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/your-org/helmix/services/auth-service/internal/config"
	githubclient "github.com/your-org/helmix/services/auth-service/internal/github"
	"github.com/your-org/helmix/services/auth-service/internal/server"
	"github.com/your-org/helmix/services/auth-service/internal/session"
	"github.com/your-org/helmix/services/auth-service/internal/store"
	vaultclient "github.com/your-org/helmix/services/auth-service/internal/vault"
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

	sessionStore, err := session.New(config.RedisURL, config.RefreshTTL)
	if err != nil {
		logger.Error("open redis", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer sessionStore.Close()

	githubAPI := githubclient.New(
		&http.Client{Timeout: config.HTTPClientTimeout},
		config.GitHubOAuthBaseURL,
		config.GitHubAPIBaseURL,
		config.GitHubClientID,
		config.GitHubClientSecret,
		config.GitHubRedirectURL,
	)

	vaultSecretsClient, err := vaultclient.NewHTTPClient(vaultclient.Config{
		Address:         config.VaultURL,
		AppRoleID:       config.VaultAppRoleID,
		AppRoleSecretID: config.VaultAppRoleSecretID,
		KVMount:         config.VaultKVMount,
	}, &http.Client{Timeout: config.HTTPClientTimeout})
	if err != nil {
		logger.Error("configure vault client", slog.String("error", err.Error()))
		os.Exit(1)
	}

	handler := server.New(config, logger, githubAPI, databaseStore, sessionStore, vaultSecretsClient)
	httpServer := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("auth-service listening", slog.String("addr", httpServer.Addr))
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("server failed: " + err.Error())
	}
}
