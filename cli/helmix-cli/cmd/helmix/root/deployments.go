package root

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func newDeploymentsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "deployments", Short: "Deployment lifecycle commands"}

	var deployRepoID, deployCommit, deployBranch, deployEnv, deployImage string
	var readyAfter int
	var simulateFailure bool
	deployCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a deployment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deployRepoID == "" || deployCommit == "" || deployBranch == "" || deployEnv == "" {
				return fmt.Errorf("repo-id, commit-sha, branch, and environment are required")
			}
			body := map[string]any{
				"repo_id":             deployRepoID,
				"commit_sha":          deployCommit,
				"branch":              deployBranch,
				"environment":         deployEnv,
				"image_tag":           deployImage,
				"ready_after_seconds": readyAfter,
				"simulate_failure":    simulateFailure,
			}
			return runJSON(cmd, http.MethodPost, "/api/v1/deployments/deploy", body, true)
		},
	}
	deployCmd.Flags().StringVar(&deployRepoID, "repo-id", "", "Repository ID")
	deployCmd.Flags().StringVar(&deployCommit, "commit-sha", "", "Commit SHA")
	deployCmd.Flags().StringVar(&deployBranch, "branch", "", "Branch name")
	deployCmd.Flags().StringVar(&deployEnv, "environment", "", "Environment name")
	deployCmd.Flags().StringVar(&deployImage, "image-tag", "", "Container image tag")
	deployCmd.Flags().IntVar(&readyAfter, "ready-after-seconds", 0, "Optional async readiness delay")
	deployCmd.Flags().BoolVar(&simulateFailure, "simulate-failure", false, "Simulate deployment failure")

	var deploymentID string
	statusCmd := &cobra.Command{
		Use:   "get",
		Short: "Get deployment status by ID",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deploymentID == "" {
				return fmt.Errorf("id is required")
			}
			path := fmt.Sprintf("/api/v1/deployments/deployments/%s", deploymentID)
			return runJSON(cmd, http.MethodGet, path, nil, true)
		},
	}
	statusCmd.Flags().StringVar(&deploymentID, "id", "", "Deployment ID")

	var projectID string
	var limit int
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List deployments by project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if projectID == "" {
				return fmt.Errorf("project-id is required")
			}
			path := fmt.Sprintf("/api/v1/deployments/deployments?project_id=%s&limit=%d", projectID, limit)
			return runJSON(cmd, http.MethodGet, path, nil, true)
		},
	}
	listCmd.Flags().StringVar(&projectID, "project-id", "", "Project ID")
	listCmd.Flags().IntVar(&limit, "limit", 10, "Maximum results")

	var rollbackID string
	rollbackCmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback a deployment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if rollbackID == "" {
				return fmt.Errorf("id is required")
			}
			path := fmt.Sprintf("/api/v1/deployments/deployments/%s/rollback", rollbackID)
			return runJSON(cmd, http.MethodPost, path, nil, true)
		},
	}
	rollbackCmd.Flags().StringVar(&rollbackID, "id", "", "Deployment ID")

	cmd.AddCommand(deployCmd, statusCmd, listCmd, rollbackCmd)
	return cmd
}
