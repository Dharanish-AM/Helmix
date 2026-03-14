package root

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func newPipelinesCmd() *cobra.Command {
	var projectSlug, provider, runtime, framework string
	cmd := &cobra.Command{Use: "pipelines", Short: "Pipeline generation commands"}
	generateCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate CI/CD pipeline templates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if projectSlug == "" || runtime == "" || framework == "" {
				return fmt.Errorf("project-slug, runtime, and framework are required")
			}
			body := map[string]any{
				"project_slug": projectSlug,
				"provider":     provider,
				"stack": map[string]any{
					"runtime":   runtime,
					"framework": framework,
				},
			}
			return runJSON(cmd, http.MethodPost, "/api/v1/pipelines/generate", body, true)
		},
	}
	generateCmd.Flags().StringVar(&projectSlug, "project-slug", "", "Project slug")
	generateCmd.Flags().StringVar(&provider, "provider", "github-actions", "Pipeline provider")
	generateCmd.Flags().StringVar(&runtime, "runtime", "", "Runtime (node, python)")
	generateCmd.Flags().StringVar(&framework, "framework", "", "Framework (nextjs, fastapi)")
	cmd.AddCommand(generateCmd)
	return cmd
}
