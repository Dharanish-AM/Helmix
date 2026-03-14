package observability

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/your-org/helmix/services/observability/internal/store"
)

const (
	sourceTypeHelmixJSON = "helmix-json"
	sourceTypePrometheus = "prometheus"
)

var prometheusMetricAliases = map[string][]string{
	"cpu_pct":         {"helmix_cpu_pct", "cpu_pct"},
	"memory_pct":      {"helmix_memory_pct", "memory_pct"},
	"req_per_sec":     {"helmix_req_per_sec", "req_per_sec"},
	"error_rate_pct":  {"helmix_error_rate_pct", "error_rate_pct"},
	"p99_latency_ms":  {"helmix_p99_latency_ms", "p99_latency_ms"},
	"pod_count":       {"helmix_pod_count", "pod_count"},
	"ready_pod_count": {"helmix_ready_pod_count", "ready_pod_count"},
	"pod_restarts":    {"helmix_pod_restarts", "pod_restarts"},
}

func (s *Service) GetTelemetrySource(ctx context.Context, projectID string) (store.TelemetrySource, error) {
	return s.store.GetTelemetrySource(ctx, strings.TrimSpace(projectID))
}

func (s *Service) UpsertTelemetrySource(ctx context.Context, params store.UpsertTelemetrySourceParams) (store.TelemetrySource, error) {
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.SourceType = strings.TrimSpace(strings.ToLower(params.SourceType))
	params.MetricsURL = strings.TrimSpace(params.MetricsURL)
	if params.ProjectID == "" {
		return store.TelemetrySource{}, fmt.Errorf("project_id is required")
	}
	if params.SourceType == "" {
		params.SourceType = sourceTypeHelmixJSON
	}
	if params.SourceType != sourceTypeHelmixJSON && params.SourceType != sourceTypePrometheus {
		return store.TelemetrySource{}, fmt.Errorf("source_type must be one of: %s, %s", sourceTypeHelmixJSON, sourceTypePrometheus)
	}
	if params.MetricsURL == "" {
		return store.TelemetrySource{}, fmt.Errorf("metrics_url is required")
	}
	if !strings.HasPrefix(params.MetricsURL, "http://") && !strings.HasPrefix(params.MetricsURL, "https://") {
		return store.TelemetrySource{}, fmt.Errorf("metrics_url must start with http:// or https://")
	}
	if params.ScrapeIntervalSeconds <= 0 {
		params.ScrapeIntervalSeconds = 30
	}
	return s.store.UpsertTelemetrySource(ctx, params)
}

func (s *Service) ScrapeTelemetrySource(ctx context.Context, projectID string) (SnapshotResponse, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return SnapshotResponse{}, fmt.Errorf("project_id is required")
	}

	source, err := s.store.GetTelemetrySource(ctx, projectID)
	if err != nil {
		return SnapshotResponse{}, err
	}
	if !source.Enabled {
		return SnapshotResponse{}, fmt.Errorf("telemetry source is disabled")
	}

	request, scrapeErr := s.scrapeTelemetryRequest(ctx, source)
	recordedAt := time.Now().UTC()
	_ = s.store.RecordTelemetryScrape(ctx, projectID, recordedAt, scrapeErr)
	if scrapeErr != nil {
		return SnapshotResponse{}, scrapeErr
	}
	request.ProjectID = projectID
	request.Source = source.SourceType + ":" + source.MetricsURL
	return s.IngestSnapshot(ctx, request)
}

func (s *Service) scrapeTelemetryRequest(ctx context.Context, source store.TelemetrySource) (SnapshotRequest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.MetricsURL, nil)
	if err != nil {
		return SnapshotRequest{}, fmt.Errorf("build telemetry request: %w", err)
	}

	response, err := s.httpClient.Do(req)
	if err != nil {
		return SnapshotRequest{}, fmt.Errorf("fetch telemetry source: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return SnapshotRequest{}, fmt.Errorf("telemetry source returned status %d", response.StatusCode)
	}

	switch source.SourceType {
	case sourceTypePrometheus:
		return parsePrometheusSnapshot(response.Body)
	default:
		return parseHelmixJSONSnapshot(response.Body)
	}
}

func parseHelmixJSONSnapshot(body io.Reader) (SnapshotRequest, error) {
	var request SnapshotRequest
	if err := json.NewDecoder(body).Decode(&request); err != nil {
		return SnapshotRequest{}, fmt.Errorf("decode telemetry json: %w", err)
	}
	if request.PodCount < 0 || request.ReadyPodCount < 0 {
		return SnapshotRequest{}, fmt.Errorf("telemetry json contains invalid pod counts")
	}
	return request, nil
}

func parsePrometheusSnapshot(body io.Reader) (SnapshotRequest, error) {
	values := map[string]float64{}
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		metricName := fields[0]
		if idx := strings.Index(metricName, "{"); idx >= 0 {
			metricName = metricName[:idx]
		}
		value, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		if err != nil {
			continue
		}
		values[metricName] = value
	}
	if err := scanner.Err(); err != nil {
		return SnapshotRequest{}, fmt.Errorf("scan prometheus payload: %w", err)
	}

	readMetric := func(key string) (float64, bool) {
		for _, alias := range prometheusMetricAliases[key] {
			if value, ok := values[alias]; ok {
				return value, true
			}
		}
		return 0, false
	}

	cpuPct, ok := readMetric("cpu_pct")
	if !ok {
		return SnapshotRequest{}, fmt.Errorf("prometheus payload missing cpu_pct gauge")
	}
	memoryPct, ok := readMetric("memory_pct")
	if !ok {
		return SnapshotRequest{}, fmt.Errorf("prometheus payload missing memory_pct gauge")
	}
	reqPerSec, ok := readMetric("req_per_sec")
	if !ok {
		return SnapshotRequest{}, fmt.Errorf("prometheus payload missing req_per_sec gauge")
	}
	errorRatePct, ok := readMetric("error_rate_pct")
	if !ok {
		return SnapshotRequest{}, fmt.Errorf("prometheus payload missing error_rate_pct gauge")
	}
	p99LatencyMS, ok := readMetric("p99_latency_ms")
	if !ok {
		return SnapshotRequest{}, fmt.Errorf("prometheus payload missing p99_latency_ms gauge")
	}
	podCount, ok := readMetric("pod_count")
	if !ok {
		return SnapshotRequest{}, fmt.Errorf("prometheus payload missing pod_count gauge")
	}
	readyPodCount, ok := readMetric("ready_pod_count")
	if !ok {
		return SnapshotRequest{}, fmt.Errorf("prometheus payload missing ready_pod_count gauge")
	}
	podRestarts, _ := readMetric("pod_restarts")

	return SnapshotRequest{
		CPUPct:        cpuPct,
		MemoryPct:     memoryPct,
		ReqPerSec:     reqPerSec,
		ErrorRatePct:  errorRatePct,
		P99LatencyMS:  p99LatencyMS,
		PodCount:      int(podCount),
		ReadyPodCount: int(readyPodCount),
		PodRestarts:   int(podRestarts),
	}, nil
}
