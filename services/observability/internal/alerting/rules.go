package alerting

import (
	"fmt"
	"sort"
	"time"
)

// MetricSnapshot is the alerting input shape.
type MetricSnapshot struct {
	ProjectID      string
	CapturedAt     time.Time
	CPUPct         float64
	MemoryPct      float64
	ReqPerSec      float64
	ErrorRatePct   float64
	P99LatencyMS   float64
	PodCount       int
	ReadyPodCount  int
	PodRestarts    int
}

// AlertCandidate is a rule breach ready to persist and publish.
type AlertCandidate struct {
	Rule        string
	Severity    string
	Metric      string
	Title       string
	Description string
	Value       float64
	Threshold   float64
}

// Evaluate returns the currently active alerts for a project's recent snapshots.
func Evaluate(snapshots []MetricSnapshot) []AlertCandidate {
	if len(snapshots) == 0 {
		return nil
	}

	sortedSnapshots := append([]MetricSnapshot(nil), snapshots...)
	sort.Slice(sortedSnapshots, func(i, j int) bool {
		return sortedSnapshots[i].CapturedAt.Before(sortedSnapshots[j].CapturedAt)
	})

	latest := sortedSnapshots[len(sortedSnapshots)-1]
	var alerts []AlertCandidate

	if latest.ReadyPodCount == 0 {
		alerts = append(alerts, newCandidate("ready-pods-zero", "critical", "ready_pod_count", latest.ProjectID, float64(latest.ReadyPodCount), 0, "ready pod count is zero"))
	}
	if latest.PodRestarts > 3 {
		alerts = append(alerts, newCandidate("pod-restarts-high", "critical", "pod_restarts", latest.ProjectID, float64(latest.PodRestarts), 3, "pod restarts exceeded threshold in the latest snapshot"))
	}
	if consecutiveCount(sortedSnapshots, func(snapshot MetricSnapshot) bool { return snapshot.CPUPct > 85 }) >= 5 {
		alerts = append(alerts, newCandidate("cpu-high", "warning", "cpu_pct", latest.ProjectID, latest.CPUPct, 85, "cpu exceeded 85% across 5 consecutive intervals"))
	}
	if consecutiveCount(sortedSnapshots, func(snapshot MetricSnapshot) bool { return snapshot.ErrorRatePct > 5 }) >= 4 {
		alerts = append(alerts, newCandidate("error-rate-high", "critical", "error_rate_pct", latest.ProjectID, latest.ErrorRatePct, 5, "error rate exceeded 5% across 4 consecutive intervals"))
	}
	if consecutiveCount(sortedSnapshots, func(snapshot MetricSnapshot) bool { return snapshot.P99LatencyMS > 2000 }) >= 6 {
		alerts = append(alerts, newCandidate("p99-latency-high", "warning", "p99_latency_ms", latest.ProjectID, latest.P99LatencyMS, 2000, "p99 latency exceeded 2000ms across 6 consecutive intervals"))
	}

	return alerts
}

func consecutiveCount(snapshots []MetricSnapshot, predicate func(MetricSnapshot) bool) int {
	count := 0
	for idx := len(snapshots) - 1; idx >= 0; idx-- {
		if !predicate(snapshots[idx]) {
			break
		}
		count++
	}
	return count
}

func newCandidate(rule, severity, metric, projectID string, value, threshold float64, detail string) AlertCandidate {
	title := fmt.Sprintf("%s on project %s", rule, projectID)
	return AlertCandidate{
		Rule:        rule,
		Severity:    severity,
		Metric:      metric,
		Title:       title,
		Description: detail,
		Value:       value,
		Threshold:   threshold,
	}
}