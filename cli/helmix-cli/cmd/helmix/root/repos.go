package root

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func newReposCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "repos", Short: "Repository onboarding and analysis"}

	var listLimit int
	var listQuery string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List connected repositories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := fmt.Sprintf("/api/v1/repos/projects?limit=%d", listLimit)
			if listQuery != "" {
				path += "&q=" + listQuery
			}
			return runJSON(cmd, http.MethodGet, path, nil, true)
		},
	}
	listCmd.Flags().IntVar(&listLimit, "limit", 20, "Max results")
	listCmd.Flags().StringVar(&listQuery, "query", "", "Filter by repository text")

	var connectRepo, connectDefaultBranch string
	connectCmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect a GitHub repository",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if connectRepo == "" {
				return fmt.Errorf("github-repo is required")
			}
			body := map[string]any{"github_repo": connectRepo, "default_branch": connectDefaultBranch}
			return runJSON(cmd, http.MethodPost, "/api/v1/repos/projects", body, true)
		},
	}
	connectCmd.Flags().StringVar(&connectRepo, "github-repo", "", "Repository in owner/name format")
	connectCmd.Flags().StringVar(&connectDefaultBranch, "default-branch", "main", "Default branch name")

	var analyzeRepoID, analyzeGitHubRepo string
	analyzeCmd := &cobra.Command{
		Use:   "analyze",
		Short: "Trigger stack analysis for a repository",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if analyzeRepoID == "" || analyzeGitHubRepo == "" {
				return fmt.Errorf("repo-id and github-repo are required")
			}
			body := map[string]any{"repo_id": analyzeRepoID, "github_repo": analyzeGitHubRepo}
			return runJSON(cmd, http.MethodPost, "/api/v1/auth/repos/analyze", body, true)
		},
	}
	analyzeCmd.Flags().StringVar(&analyzeRepoID, "repo-id", "", "Repository ID from connect/list")
	analyzeCmd.Flags().StringVar(&analyzeGitHubRepo, "github-repo", "", "Repository in owner/name format")

	cmd.AddCommand(listCmd, connectCmd, analyzeCmd)
	return cmd
}
