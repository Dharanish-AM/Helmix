package root

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

func newIncidentsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "incidents", Short: "Incident AI operations"}

	var projectID string
	var limit, offset int
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List incidents for a project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if projectID == "" {
				return fmt.Errorf("project-id is required")
			}
			path := fmt.Sprintf("/api/v1/incidents/projects/%s?limit=%d&offset=%d", projectID, limit, offset)
			return runJSON(cmd, http.MethodGet, path, nil, true)
		},
	}
	listCmd.Flags().StringVar(&projectID, "project-id", "", "Project ID")
	listCmd.Flags().IntVar(&limit, "limit", 20, "Max items")
	listCmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")

	var incidentID string
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get incident details",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if incidentID == "" {
				return fmt.Errorf("id is required")
			}
			path := fmt.Sprintf("/api/v1/incidents/%s", incidentID)
			return runJSON(cmd, http.MethodGet, path, nil, true)
		},
	}
	getCmd.Flags().StringVar(&incidentID, "id", "", "Incident ID")

	similarCmd := &cobra.Command{
		Use:   "similar",
		Short: "Get similar incidents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if incidentID == "" {
				return fmt.Errorf("id is required")
			}
			path := fmt.Sprintf("/api/v1/incidents/%s/similar", incidentID)
			return runJSON(cmd, http.MethodGet, path, nil, true)
		},
	}
	similarCmd.Flags().StringVar(&incidentID, "id", "", "Incident ID")

	var action, deploymentID string
	actionCmd := &cobra.Command{
		Use:   "action",
		Short: "Trigger manual action on an incident",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if incidentID == "" || action == "" {
				return fmt.Errorf("id and action are required")
			}
			body := map[string]any{"action": action, "params": map[string]any{}}
			if deploymentID != "" {
				body["params"] = map[string]any{"deployment_id": deploymentID}
			}
			path := fmt.Sprintf("/api/v1/incidents/%s/actions", incidentID)
			return runJSON(cmd, http.MethodPost, path, body, true)
		},
	}
	actionCmd.Flags().StringVar(&incidentID, "id", "", "Incident ID")
	actionCmd.Flags().StringVar(&action, "action", "", "Action to execute")
	actionCmd.Flags().StringVar(&deploymentID, "deployment-id", "", "Optional deployment ID parameter")

	cmd.AddCommand(listCmd, getCmd, similarCmd, actionCmd)
	return cmd
}
