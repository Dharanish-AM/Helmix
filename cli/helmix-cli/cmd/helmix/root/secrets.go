package root

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func newSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "secrets", Short: "Vault-backed secret management"}

	var setService, setKey, setValue string
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Create or update a secret",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if setService == "" || setKey == "" || setValue == "" {
				return fmt.Errorf("service, key, and value are required")
			}
			body := map[string]any{"service": setService, "key": setKey, "value": setValue}
			return runJSON(cmd, http.MethodPost, "/api/v1/secrets", body, true)
		},
	}
	setCmd.Flags().StringVar(&setService, "service", "", "Service name")
	setCmd.Flags().StringVar(&setKey, "key", "", "Secret key")
	setCmd.Flags().StringVar(&setValue, "value", "", "Secret value")

	var getService, getKey string
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Read a secret",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if getService == "" || getKey == "" {
				return fmt.Errorf("service and key are required")
			}
			path := fmt.Sprintf("/api/v1/secrets/%s/%s", getService, getKey)
			return runJSON(cmd, http.MethodGet, path, nil, true)
		},
	}
	getCmd.Flags().StringVar(&getService, "service", "", "Service name")
	getCmd.Flags().StringVar(&getKey, "key", "", "Secret key")

	var delService, delKey string
	delCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a secret",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if delService == "" || delKey == "" {
				return fmt.Errorf("service and key are required")
			}
			path := fmt.Sprintf("/api/v1/secrets/%s/%s", delService, delKey)
			return runJSON(cmd, http.MethodDelete, path, nil, true)
		},
	}
	delCmd.Flags().StringVar(&delService, "service", "", "Service name")
	delCmd.Flags().StringVar(&delKey, "key", "", "Secret key")

	cmd.AddCommand(setCmd, getCmd, delCmd)
	return cmd
}
