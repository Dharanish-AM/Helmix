package root

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

func newInfraCmd() *cobra.Command {
	var projectSlug, provider, runtime, framework, database string
	var port int
	cmd := &cobra.Command{
		Use:   "infra",
		Short: "Infrastructure template generation",
	}
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate infra templates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if projectSlug == "" || runtime == "" || framework == "" {
				return fmt.Errorf("project-slug, runtime, and framework are required")
			}
			stack := map[string]any{"runtime": runtime, "framework": framework}
			if port > 0 {
				stack["port"] = port
			}
			if strings.TrimSpace(database) != "" {
				stack["database"] = strings.Split(database, ",")
			}
			body := map[string]any{"project_slug": projectSlug, "provider": provider, "stack": stack}
			return runJSON(cmd, http.MethodPost, "/api/v1/infra/generate", body, true)
		},
	}
	generateCmd.Flags().StringVar(&projectSlug, "project-slug", "", "Project slug")
	generateCmd.Flags().StringVar(&provider, "provider", "docker", "Infrastructure provider")
	generateCmd.Flags().StringVar(&runtime, "runtime", "", "Runtime (node, python)")
	generateCmd.Flags().StringVar(&framework, "framework", "", "Framework (nextjs, fastapi)")
	generateCmd.Flags().StringVar(&database, "database", "", "Comma-separated databases")
	generateCmd.Flags().IntVar(&port, "port", 0, "Application port")

	cmd.AddCommand(generateCmd)
	return cmd
}
