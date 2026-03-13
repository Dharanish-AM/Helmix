package eventsdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// EventType is the canonical subject name for Helmix events.
type EventType string

const (
	RepoConnected        EventType = "repo.connected"
	RepoAnalyzed         EventType = "repo.analyzed"
	InfraGenerated       EventType = "infra.generated"
	PipelineCreated      EventType = "pipeline.created"
	DeploymentStarted    EventType = "deployment.started"
	DeploymentSucceeded  EventType = "deployment.succeeded"
	DeploymentFailed     EventType = "deployment.failed"
	AlertFired           EventType = "alert.fired"
	IncidentCreated      EventType = "incident.created"
	IncidentResolved     EventType = "incident.resolved"
	AutoHealTriggered    EventType = "autoheal.triggered"
)

// BaseEvent contains common fields for all event payloads.
type BaseEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	OrgID     string    `json:"org_id"`
	ProjectID string    `json:"project_id"`
	CreatedAt time.Time `json:"created_at"`
}

// DetectedStack contains stack metadata derived from repository analysis.
type DetectedStack struct {
	Runtime      string   `json:"runtime"`
	Framework    string   `json:"framework"`
	Database     []string `json:"database"`
	Containerized bool    `json:"containerized"`
	HasTests     bool     `json:"has_tests"`
	Port         int      `json:"port"`
	BuildCommand string   `json:"build_command"`
	TestCommand  string   `json:"test_command"`
}

// RepoAnalyzedEvent is emitted when repository stack analysis completes.
type RepoAnalyzedEvent struct {
	BaseEvent
	RepoID string        `json:"repo_id"`
	Stack  DetectedStack `json:"stack"`
}

// DeploymentEvent is emitted for deployment lifecycle updates.
type DeploymentEvent struct {
	BaseEvent
	DeploymentID string `json:"deployment_id"`
	CommitSHA    string `json:"commit_sha"`
	Environment  string `json:"environment"`
	ImageTag     string `json:"image_tag"`
}

// AlertFiredEvent is emitted when an alert threshold has been breached.
type AlertFiredEvent struct {
	BaseEvent
	AlertID    string  `json:"alert_id"`
	Severity   string  `json:"severity"`
	Metric     string  `json:"metric"`
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold"`
}

// Publish marshals an event and publishes it to NATS using event.type as subject.
func Publish(nc *nats.Conn, event interface{}) error {
	if nc == nil {
		return errors.New("nats connection is nil")
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	eventType, err := extractEventType(payload)
	if err != nil {
		return fmt.Errorf("extract event type: %w", err)
	}

	if err := nc.Publish(eventType, payload); err != nil {
		return fmt.Errorf("publish event: %w", err)
	}

	return nil
}

// Subscribe subscribes to a NATS subject and decodes each message into T.
func Subscribe[T any](nc *nats.Conn, subject string, handler func(T)) error {
	if nc == nil {
		return errors.New("nats connection is nil")
	}
	if strings.TrimSpace(subject) == "" {
		return errors.New("subject is required")
	}
	if handler == nil {
		return errors.New("handler is required")
	}

	_, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		var event T
		if unmarshalErr := json.Unmarshal(msg.Data, &event); unmarshalErr != nil {
			return
		}
		handler(event)
	})
	if err != nil {
		return fmt.Errorf("subscribe subject %q: %w", subject, err)
	}

	return nil
}

func extractEventType(payload []byte) (string, error) {
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return "", fmt.Errorf("unmarshal payload: %w", err)
	}

	typeValue, ok := envelope["type"]
	if !ok {
		return "", errors.New("missing type field")
	}

	eventType, ok := typeValue.(string)
	if !ok || strings.TrimSpace(eventType) == "" {
		return "", errors.New("type must be a non-empty string")
	}

	return eventType, nil
}
