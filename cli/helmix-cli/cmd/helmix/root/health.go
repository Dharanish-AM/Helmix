package root

import (
	"net/http"

	"github.com/spf13/cobra"
)

func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check gateway health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runJSON(cmd, http.MethodGet, "/health", nil, false)
		},
	}
}
