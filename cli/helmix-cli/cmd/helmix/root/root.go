package root

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type appConfig struct {
	APIBaseURL string
	Token      string
	OrgID      string
	Timeout    time.Duration
	Output     string
}

type app struct {
	cfg appConfig
}

var state = &app{}

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "helmix",
		Short: "Helmix platform CLI",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Name() == "help" {
				return nil
			}
			return initConfig(cmd.Flags(), &state.cfg)
		},
	}

	cmd.PersistentFlags().String("api-base-url", "http://localhost:8080", "Helmix API gateway base URL")
	cmd.PersistentFlags().String("token", "", "Bearer token for API authentication")
	cmd.PersistentFlags().String("org-id", "", "Organization ID context sent as X-Helmix-Org-ID")
	cmd.PersistentFlags().Duration("timeout", 15*time.Second, "HTTP timeout")
	cmd.PersistentFlags().String("output", "json", "Output format: json")

	must(cmd.RegisterFlagCompletionFunc("output", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"json"}, cobra.ShellCompDirectiveNoFileComp
	}))

	cmd.AddCommand(
		newHealthCmd(),
		newAuthCmd(),
		newOrgsCmd(),
		newSecretsCmd(),
		newReposCmd(),
		newInfraCmd(),
		newPipelinesCmd(),
		newDeploymentsCmd(),
		newObservabilityCmd(),
		newIncidentsCmd(),
	)

	return cmd
}

func initConfig(flags *pflag.FlagSet, cfg *appConfig) error {
	v := viper.New()
	v.SetEnvPrefix("HELMIX")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	v.SetConfigName("helmix")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "helmix"))
	}

	_ = v.ReadInConfig()

	must(v.BindPFlag("api-base-url", flags.Lookup("api-base-url")))
	must(v.BindPFlag("token", flags.Lookup("token")))
	must(v.BindPFlag("org-id", flags.Lookup("org-id")))
	must(v.BindPFlag("timeout", flags.Lookup("timeout")))
	must(v.BindPFlag("output", flags.Lookup("output")))

	cfg.APIBaseURL = strings.TrimRight(strings.TrimSpace(v.GetString("api-base-url")), "/")
	cfg.Token = strings.TrimSpace(v.GetString("token"))
	cfg.OrgID = strings.TrimSpace(v.GetString("org-id"))
	cfg.Timeout = v.GetDuration("timeout")
	cfg.Output = strings.ToLower(strings.TrimSpace(v.GetString("output")))

	if cfg.APIBaseURL == "" {
		return fmt.Errorf("api-base-url is required")
	}
	if _, err := url.Parse(cfg.APIBaseURL); err != nil {
		return fmt.Errorf("invalid api-base-url: %w", err)
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be > 0")
	}
	if cfg.Output != "json" {
		return fmt.Errorf("unsupported output format %q", cfg.Output)
	}
	return nil
}

func requireToken() error {
	if state.cfg.Token == "" {
		return fmt.Errorf("token required: set --token or HELMIX_TOKEN")
	}
	return nil
}

func request(ctx context.Context, method, path string, body any, auth bool) (int, []byte, error) {
	endpoint := state.cfg.APIBaseURL + path
	var requestBody io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal request: %w", err)
		}
		requestBody = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, requestBody)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if state.cfg.OrgID != "" {
		req.Header.Set("X-Helmix-Org-ID", state.cfg.OrgID)
	}
	if auth {
		if err := requireToken(); err != nil {
			return 0, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+state.cfg.Token)
	}

	client := &http.Client{Timeout: state.cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, responseBody, nil
}

func printJSON(raw []byte) error {
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		fmt.Println(string(raw))
		return nil
	}
	formatted, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(formatted))
	return nil
}

func runJSON(cmd *cobra.Command, method, path string, body any, auth bool) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), state.cfg.Timeout)
	defer cancel()
	status, responseBody, err := request(ctx, method, path, body, auth)
	if err != nil {
		return err
	}
	if status >= http.StatusBadRequest {
		_ = printJSON(responseBody)
		return fmt.Errorf("request failed with status %d", status)
	}
	return printJSON(responseBody)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
