package observability

import (
	"strings"
	"testing"
)

func TestParseHelmixJSONSnapshot(t *testing.T) {
	payload := `{"cpu_pct":35,"memory_pct":45,"req_per_sec":20,"error_rate_pct":1,"p99_latency_ms":2800,"pod_count":2,"ready_pod_count":2,"pod_restarts":0}`
	snapshot, err := parseHelmixJSONSnapshot(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("parseHelmixJSONSnapshot returned error: %v", err)
	}
	if snapshot.CPUPct != 35 || snapshot.P99LatencyMS != 2800 || snapshot.PodCount != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestParsePrometheusSnapshot(t *testing.T) {
	payload := `
# HELP helmix_cpu_pct CPU usage percent
# TYPE helmix_cpu_pct gauge
helmix_cpu_pct 35
helmix_memory_pct 45
helmix_req_per_sec 20
helmix_error_rate_pct 1
helmix_p99_latency_ms 2800
helmix_pod_count 2
helmix_ready_pod_count 2
helmix_pod_restarts 0
`
	snapshot, err := parsePrometheusSnapshot(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("parsePrometheusSnapshot returned error: %v", err)
	}
	if snapshot.CPUPct != 35 || snapshot.MemoryPct != 45 || snapshot.ReadyPodCount != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}
