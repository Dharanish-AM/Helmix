package alerting

import (
	"testing"
	"time"
)

func TestCPUAlertFires(t *testing.T) {
	snapshots := buildSnapshots(5, func(snapshot *MetricSnapshot, index int) {
		snapshot.CPUPct = 92
	})
	alerts := Evaluate(snapshots)
	if !hasRule(alerts, "cpu-high") {
		t.Fatal("expected cpu-high alert")
	}
}

func TestCPUAlertNoFire(t *testing.T) {
	snapshots := buildSnapshots(3, func(snapshot *MetricSnapshot, index int) {
		snapshot.CPUPct = 92
	})
	alerts := Evaluate(snapshots)
	if hasRule(alerts, "cpu-high") {
		t.Fatal("did not expect cpu-high alert")
	}
}

func TestErrorRateCritical(t *testing.T) {
	snapshots := buildSnapshots(4, func(snapshot *MetricSnapshot, index int) {
		snapshot.ErrorRatePct = 9
	})
	alerts := Evaluate(snapshots)
	if !hasRule(alerts, "error-rate-high") {
		t.Fatal("expected error-rate-high alert")
	}
}

func TestZeroPodImmediate(t *testing.T) {
	snapshots := buildSnapshots(1, func(snapshot *MetricSnapshot, index int) {
		snapshot.PodCount = 2
		snapshot.ReadyPodCount = 0
	})
	alerts := Evaluate(snapshots)
	if !hasRule(alerts, "ready-pods-zero") {
		t.Fatal("expected ready-pods-zero alert")
	}
}

func TestAlertDeduplication(t *testing.T) {
	snapshots := buildSnapshots(6, func(snapshot *MetricSnapshot, index int) {
		snapshot.CPUPct = 92
	})
	alerts := Evaluate(snapshots)
	count := 0
	for _, alert := range alerts {
		if alert.Rule == "cpu-high" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one cpu-high alert, got %d", count)
	}
}

// --- p99 latency rule ---

func TestP99LatencyHighFires(t *testing.T) {
	snapshots := buildSnapshots(6, func(snapshot *MetricSnapshot, _ int) {
		snapshot.P99LatencyMS = 2500
	})
	alerts := Evaluate(snapshots)
	if !hasRule(alerts, "p99-latency-high") {
		t.Fatal("expected p99-latency-high alert with 6 consecutive breaching snapshots")
	}
}

func TestP99LatencyHighExactThreshold(t *testing.T) {
	// exactly 2000 ms does not exceed 2000 — should not fire
	snapshots := buildSnapshots(6, func(snapshot *MetricSnapshot, _ int) {
		snapshot.P99LatencyMS = 2000
	})
	alerts := Evaluate(snapshots)
	if hasRule(alerts, "p99-latency-high") {
		t.Fatal("expected no p99-latency-high alert at exactly the threshold value")
	}
}

func TestP99LatencyHighNoFireFewSnapshots(t *testing.T) {
	// only 5 consecutive breaches — needs 6 to fire
	snapshots := buildSnapshots(5, func(snapshot *MetricSnapshot, _ int) {
		snapshot.P99LatencyMS = 2500
	})
	alerts := Evaluate(snapshots)
	if hasRule(alerts, "p99-latency-high") {
		t.Fatal("expected no p99-latency-high alert with only 5 consecutive breaching snapshots")
	}
}

func TestP99LatencyHighNoFireGap(t *testing.T) {
	// 6 snapshots but the oldest one is below threshold — streak broken
	snapshots := buildSnapshots(6, func(snapshot *MetricSnapshot, index int) {
		if index == 0 {
			snapshot.P99LatencyMS = 100 // healthy oldest snapshot breaks the streak
		} else {
			snapshot.P99LatencyMS = 2500
		}
	})
	alerts := Evaluate(snapshots)
	if hasRule(alerts, "p99-latency-high") {
		t.Fatal("expected no p99-latency-high alert when streak is broken by an early healthy snapshot")
	}
}

func TestP99LatencySeverityIsWarning(t *testing.T) {
	snapshots := buildSnapshots(6, func(snapshot *MetricSnapshot, _ int) {
		snapshot.P99LatencyMS = 2500
	})
	alerts := Evaluate(snapshots)
	for _, alert := range alerts {
		if alert.Rule == "p99-latency-high" && alert.Severity != "warning" {
			t.Fatalf("expected p99-latency-high severity=warning, got %q", alert.Severity)
		}
	}
}

// --- ready-pods-zero rule ---

func TestZeroPodSeverityIsCritical(t *testing.T) {
	snapshots := buildSnapshots(1, func(snapshot *MetricSnapshot, _ int) {
		snapshot.PodCount = 3
		snapshot.ReadyPodCount = 0
	})
	alerts := Evaluate(snapshots)
	if !hasRule(alerts, "ready-pods-zero") {
		t.Fatal("expected ready-pods-zero alert")
	}
	for _, alert := range alerts {
		if alert.Rule == "ready-pods-zero" && alert.Severity != "critical" {
			t.Fatalf("expected ready-pods-zero severity=critical, got %q", alert.Severity)
		}
	}
}

func TestZeroPodPartialPodsHealthy(t *testing.T) {
	// at least one ready pod — rule must NOT fire
	snapshots := buildSnapshots(1, func(snapshot *MetricSnapshot, _ int) {
		snapshot.PodCount = 3
		snapshot.ReadyPodCount = 1
	})
	alerts := Evaluate(snapshots)
	if hasRule(alerts, "ready-pods-zero") {
		t.Fatal("expected no ready-pods-zero alert when at least one pod is ready")
	}
}

func TestZeroPodFiresImmediatelyWithoutHistory(t *testing.T) {
	// single snapshot is enough for ready-pods-zero — no consecutive window required
	snapshots := buildSnapshots(1, func(snapshot *MetricSnapshot, _ int) {
		snapshot.ReadyPodCount = 0
	})
	alerts := Evaluate(snapshots)
	if !hasRule(alerts, "ready-pods-zero") {
		t.Fatal("expected ready-pods-zero to fire on a single snapshot")
	}
}

func TestZeroPodValueAndThresholdInAlert(t *testing.T) {
	snapshots := buildSnapshots(1, func(snapshot *MetricSnapshot, _ int) {
		snapshot.PodCount = 2
		snapshot.ReadyPodCount = 0
	})
	alerts := Evaluate(snapshots)
	for _, alert := range alerts {
		if alert.Rule != "ready-pods-zero" {
			continue
		}
		if alert.Value != 0 {
			t.Fatalf("expected alert.Value=0, got %v", alert.Value)
		}
		if alert.Threshold != 0 {
			t.Fatalf("expected alert.Threshold=0, got %v", alert.Threshold)
		}
	}
}

func buildSnapshots(count int, mutate func(*MetricSnapshot, int)) []MetricSnapshot {
	snapshots := make([]MetricSnapshot, 0, count)
	baseTime := time.Now().Add(-time.Duration(count) * 30 * time.Second)
	for index := 0; index < count; index++ {
		snapshot := MetricSnapshot{
			ProjectID:     "project-1",
			CapturedAt:    baseTime.Add(time.Duration(index) * 30 * time.Second),
			CPUPct:        40,
			MemoryPct:     50,
			ReqPerSec:     10,
			ErrorRatePct:  1,
			P99LatencyMS:  300,
			PodCount:      2,
			ReadyPodCount: 2,
			PodRestarts:   0,
		}
		mutate(&snapshot, index)
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func hasRule(alerts []AlertCandidate, rule string) bool {
	for _, alert := range alerts {
		if alert.Rule == rule {
			return true
		}
	}
	return false
}