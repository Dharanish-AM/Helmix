package root

import (
	"net/http"

	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authentication/session commands"}
	cmd.AddCommand(&cobra.Command{
		Use:   "me",
		Short: "Show current authenticated user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runJSON(cmd, http.MethodGet, "/api/v1/auth/me", nil, true)
		},
	})
	return cmd
}
