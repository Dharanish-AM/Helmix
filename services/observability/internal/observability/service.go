package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	eventsdk "github.com/your-org/helmix/libs/event-sdk"
	"github.com/your-org/helmix/services/observability/internal/alerting"
	"github.com/your-org/helmix/services/observability/internal/store"
)

var (
	snapshotsIngested = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "helmix",
		Subsystem: "observability",
		Name:      "snapshots_ingested_total",
		Help:      "Number of metric snapshots accepted by the observability service.",
	})
	alertsFired = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "helmix",
		Subsystem: "observability",
		Name:      "alerts_fired_total",
		Help:      "Number of alerts emitted by the observability service.",
	})
)

func init() {
	prometheus.MustRegister(snapshotsIngested, alertsFired)
}

// SnapshotRequest is the API contract for synthetic or scraped metrics.
type SnapshotRequest struct {
	ProjectID     string     `json:"project_id"`
	CapturedAt    *time.Time `json:"captured_at,omitempty"`
	CPUPct        float64    `json:"cpu_pct"`
	MemoryPct     float64    `json:"memory_pct"`
	ReqPerSec     float64    `json:"req_per_sec"`
	ErrorRatePct  float64    `json:"error_rate_pct"`
	P99LatencyMS  float64    `json:"p99_latency_ms"`
	PodCount      int        `json:"pod_count"`
	ReadyPodCount int        `json:"ready_pod_count"`
	PodRestarts   int        `json:"pod_restarts,omitempty"`
	Source        string     `json:"source,omitempty"`
}

// SnapshotResponse returns the persisted snapshot with alert side effects.
type SnapshotResponse struct {
	Snapshot    store.MetricSnapshot `json:"snapshot"`
	AlertsFired []store.Alert        `json:"alerts_fired"`
}

// Service exposes the observability use cases.
type Service struct {
	logger     *slog.Logger
	store      *store.Store
	publisher  Publisher
	httpClient *http.Client
}

func NewService(logger *slog.Logger, observabilityStore *store.Store, publisher Publisher) *Service {
	return &Service{
		logger:     logger,
		store:      observabilityStore,
		publisher:  publisher,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Service) IngestSnapshot(ctx context.Context, request SnapshotRequest) (SnapshotResponse, error) {
	if strings.TrimSpace(request.ProjectID) == "" {
		return SnapshotResponse{}, fmt.Errorf("project_id is required")
	}
	capturedAt := time.Now().UTC()
	if request.CapturedAt != nil {
		capturedAt = request.CapturedAt.UTC()
	}

	snapshot, err := s.store.CreateSnapshot(ctx, store.CreateSnapshotParams{
		ProjectID:     strings.TrimSpace(request.ProjectID),
		CapturedAt:    capturedAt,
		CPUPct:        request.CPUPct,
		MemoryPct:     request.MemoryPct,
		ReqPerSec:     request.ReqPerSec,
		ErrorRatePct:  request.ErrorRatePct,
		P99LatencyMS:  request.P99LatencyMS,
		PodCount:      request.PodCount,
		ReadyPodCount: request.ReadyPodCount,
		PodRestarts:   request.PodRestarts,
		Source:        request.Source,
	})
	if err != nil {
		return SnapshotResponse{}, err
	}
	snapshotsIngested.Inc()

	recentSnapshots, err := s.store.ListSnapshots(ctx, snapshot.ProjectID, capturedAt.Add(-24*time.Hour))
	if err != nil {
		return SnapshotResponse{}, fmt.Errorf("list recent snapshots: %w", err)
	}

	alertInputs := make([]alerting.MetricSnapshot, 0, len(recentSnapshots))
	for _, item := range recentSnapshots {
		alertInputs = append(alertInputs, alerting.MetricSnapshot{
			ProjectID:     item.ProjectID,
			CapturedAt:    item.CapturedAt,
			CPUPct:        item.CPUPct,
			MemoryPct:     item.MemoryPct,
			ReqPerSec:     item.ReqPerSec,
			ErrorRatePct:  item.ErrorRatePct,
			P99LatencyMS:  item.P99LatencyMS,
			PodCount:      item.PodCount,
			ReadyPodCount: item.ReadyPodCount,
			PodRestarts:   item.PodRestarts,
		})
	}

	candidates := alerting.Evaluate(alertInputs)
	activeTitles := make([]string, 0, len(candidates))
	firedAlerts := make([]store.Alert, 0, len(candidates))
	orgID, _ := s.store.GetProjectContext(ctx, snapshot.ProjectID)

	for _, candidate := range candidates {
		activeTitles = append(activeTitles, candidate.Title)
		existingAlert, err := s.store.FindOpenAlertByTitle(ctx, snapshot.ProjectID, candidate.Title)
		if err != nil {
			return SnapshotResponse{}, fmt.Errorf("find open alert: %w", err)
		}
		if existingAlert != nil {
			continue
		}

		createdAlert, err := s.store.CreateAlert(ctx, store.CreateAlertParams{
			ProjectID:   snapshot.ProjectID,
			Rule:        candidate.Rule,
			Severity:    candidate.Severity,
			Title:       candidate.Title,
			Description: candidate.Description,
			Metric:      candidate.Metric,
			Value:       candidate.Value,
			Threshold:   candidate.Threshold,
			FiredAt:     snapshot.CapturedAt,
		})
		if err != nil {
			return SnapshotResponse{}, fmt.Errorf("create alert: %w", err)
		}
		firedAlerts = append(firedAlerts, createdAlert)
		alertsFired.Inc()

		event := eventsdk.AlertFiredEvent{
			BaseEvent: eventsdk.BaseEvent{
				ID:        createdAlert.ID,
				Type:      string(eventsdk.AlertFired),
				OrgID:     orgID,
				ProjectID: snapshot.ProjectID,
				CreatedAt: time.Now().UTC(),
			},
			AlertID:   createdAlert.ID,
			Severity:  createdAlert.Severity,
			Metric:    candidate.Metric,
			Value:     candidate.Value,
			Threshold: candidate.Threshold,
		}
		if publishErr := s.publisher.Publish(ctx, event); publishErr != nil {
			s.logger.Warn("publish alert.fired failed", slog.String("alert_id", createdAlert.ID), slog.String("error", publishErr.Error()))
		}
	}

	if err := s.store.ResolveAlertsExcept(ctx, snapshot.ProjectID, activeTitles); err != nil {
		return SnapshotResponse{}, fmt.Errorf("resolve inactive alerts: %w", err)
	}

	return SnapshotResponse{Snapshot: snapshot, AlertsFired: firedAlerts}, nil
}

func (s *Service) ListSnapshots(ctx context.Context, projectID string) ([]store.MetricSnapshot, error) {
	return s.store.ListSnapshots(ctx, strings.TrimSpace(projectID), time.Now().Add(-24*time.Hour))
}

func (s *Service) CurrentSnapshot(ctx context.Context, projectID string) (store.MetricSnapshot, error) {
	return s.store.GetLatestSnapshot(ctx, strings.TrimSpace(projectID))
}

func (s *Service) OpenAlerts(ctx context.Context, projectID string) ([]store.Alert, error) {
	return s.store.ListOpenAlerts(ctx, strings.TrimSpace(projectID))
}
