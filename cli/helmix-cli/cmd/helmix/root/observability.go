package root

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func newObservabilityCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "observability", Short: "Metrics and alerts commands"}

	var projectID string
	currentCmd := &cobra.Command{
		Use:   "current",
		Short: "Fetch current metric snapshot",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if projectID == "" {
				return fmt.Errorf("project-id is required")
			}
			path := fmt.Sprintf("/api/v1/observability/metrics/%s/current", projectID)
			return runJSON(cmd, http.MethodGet, path, nil, true)
		},
	}
	currentCmd.Flags().StringVar(&projectID, "project-id", "", "Project ID")

	alertsCmd := &cobra.Command{
		Use:   "alerts",
		Short: "List open alerts for a project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if projectID == "" {
				return fmt.Errorf("project-id is required")
			}
			path := fmt.Sprintf("/api/v1/observability/alerts/%s", projectID)
			return runJSON(cmd, http.MethodGet, path, nil, true)
		},
	}
	alertsCmd.Flags().StringVar(&projectID, "project-id", "", "Project ID")

	var cpuPct, memoryPct, reqPerSec, errorRatePct, p99Latency float64
	var podCount, readyPodCount, podRestarts int
	ingestCmd := &cobra.Command{
		Use:   "ingest",
		Short: "Ingest a metric snapshot",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if projectID == "" {
				return fmt.Errorf("project-id is required")
			}
			body := map[string]any{
				"project_id":      projectID,
				"captured_at":     time.Now().UTC().Format(time.RFC3339),
				"cpu_pct":         cpuPct,
				"memory_pct":      memoryPct,
				"req_per_sec":     reqPerSec,
				"error_rate_pct":  errorRatePct,
				"p99_latency_ms":  p99Latency,
				"pod_count":       podCount,
				"ready_pod_count": readyPodCount,
				"pod_restarts":    podRestarts,
				"source":          "helmix-cli",
			}
			return runJSON(cmd, http.MethodPost, "/api/v1/observability/snapshots", body, true)
		},
	}
	ingestCmd.Flags().StringVar(&projectID, "project-id", "", "Project ID")
	ingestCmd.Flags().Float64Var(&cpuPct, "cpu-pct", 0, "CPU utilization percentage")
	ingestCmd.Flags().Float64Var(&memoryPct, "memory-pct", 0, "Memory utilization percentage")
	ingestCmd.Flags().Float64Var(&reqPerSec, "req-per-sec", 0, "Requests per second")
	ingestCmd.Flags().Float64Var(&errorRatePct, "error-rate-pct", 0, "Error rate percentage")
	ingestCmd.Flags().Float64Var(&p99Latency, "p99-latency-ms", 0, "p99 latency in ms")
	ingestCmd.Flags().IntVar(&podCount, "pod-count", 0, "Total pod count")
	ingestCmd.Flags().IntVar(&readyPodCount, "ready-pod-count", 0, "Ready pod count")
	ingestCmd.Flags().IntVar(&podRestarts, "pod-restarts", 0, "Total pod restarts")

	cmd.AddCommand(currentCmd, alertsCmd, ingestCmd)
	return cmd
}
