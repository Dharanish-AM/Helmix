package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	eventsdk "github.com/your-org/helmix/libs/event-sdk"
	"github.com/your-org/helmix/services/deployment-engine/internal/store"
)

func TestBlueGreenHappyPath(t *testing.T) {
	fakeStore := newFakeStore()
	previous := fakeStore.seedDeployment(store.Deployment{
		ID:          "dep-blue",
		RepoID:      "repo-1",
		CommitSHA:   "sha-old",
		Branch:      "main",
		Status:      "live",
		Environment: "production",
		ImageTag:    "ghcr.io/acme/app:old",
		CreatedAt:   time.Now().Add(-2 * time.Minute).UTC(),
		DeployedAt:  timePtr(time.Now().Add(-90 * time.Second).UTC()),
	})
	publisher := &fakePublisher{}
	service := NewService(testLogger(), fakeStore, publisher, 250*time.Millisecond, 25*time.Millisecond)

	response, err := service.StartDeployment(context.Background(), "org-1", StartRequest{
		RepoID:      previous.RepoID,
		CommitSHA:   "sha-new",
		Branch:      "main",
		AcceptRisk:  true,
		ScanResults: map[string]any{"critical": 1},
		Environment: "production",
		ImageTag:    "ghcr.io/acme/app:new",
	})
	if err != nil {
		t.Fatalf("start deployment failed: %v", err)
	}

	assertEventually(t, time.Second, func() error {
		current, err := service.GetDeployment(context.Background(), response.ID)
		if err != nil {
			return err
		}
		if current.Status != "live" {
			return fmt.Errorf("status=%s", current.Status)
		}
		if !current.Active {
			return errors.New("expected deployment to be active")
		}
		if current.PreviousDeploymentID != previous.ID {
			return fmt.Errorf("previous=%s", current.PreviousDeploymentID)
		}
		seededPrevious, _ := fakeStore.GetDeployment(context.Background(), previous.ID)
		if seededPrevious.Status != "superseded" {
			return fmt.Errorf("previous status=%s", seededPrevious.Status)
		}
		return nil
	})

	if !publisher.sawEvent(string(eventsdk.DeploymentStarted)) {
		t.Fatal("expected deployment.started event")
	}
	if !publisher.sawEvent(string(eventsdk.DeploymentSucceeded)) {
		t.Fatal("expected deployment.succeeded event")
	}
}

func TestBlueGreenTimeoutTriggersRollback(t *testing.T) {
	fakeStore := newFakeStore()
	fakeStore.seedDeployment(store.Deployment{
		ID:          "dep-blue",
		RepoID:      "repo-1",
		CommitSHA:   "sha-old",
		Branch:      "main",
		Status:      "live",
		Environment: "production",
		ImageTag:    "ghcr.io/acme/app:old",
		CreatedAt:   time.Now().Add(-2 * time.Minute).UTC(),
		DeployedAt:  timePtr(time.Now().Add(-90 * time.Second).UTC()),
	})
	service := NewService(testLogger(), fakeStore, &fakePublisher{}, 30*time.Millisecond, 80*time.Millisecond)

	response, err := service.StartDeployment(context.Background(), "org-1", StartRequest{
		RepoID:      "repo-1",
		CommitSHA:   "sha-timeout",
		Branch:      "main",
		AcceptRisk:  true,
		ScanResults: map[string]any{"high": 1},
		Environment: "production",
		ImageTag:    "ghcr.io/acme/app:timeout",
	})
	if err != nil {
		t.Fatalf("start deployment failed: %v", err)
	}

	assertEventually(t, time.Second, func() error {
		current, err := service.GetDeployment(context.Background(), response.ID)
		if err != nil {
			return err
		}
		if current.Status != "failed" {
			return fmt.Errorf("status=%s", current.Status)
		}
		if current.Active {
			return errors.New("failed deployment must not be active")
		}
		if current.CurrentLiveDeployment == response.ID {
			return errors.New("failed deployment should not remain current live")
		}
		return nil
	})
}

func TestRollbackRestoresPreviousImage(t *testing.T) {
	fakeStore := newFakeStore()
	previous := fakeStore.seedDeployment(store.Deployment{
		ID:          "dep-blue",
		RepoID:      "repo-1",
		CommitSHA:   "sha-old",
		Branch:      "main",
		Status:      "superseded",
		Environment: "production",
		ImageTag:    "ghcr.io/acme/app:old",
		CreatedAt:   time.Now().Add(-5 * time.Minute).UTC(),
		DeployedAt:  timePtr(time.Now().Add(-4 * time.Minute).UTC()),
	})
	current := fakeStore.seedDeployment(store.Deployment{
		ID:          "dep-green",
		RepoID:      "repo-1",
		CommitSHA:   "sha-new",
		Branch:      "main",
		Status:      "live",
		Environment: "production",
		ImageTag:    "ghcr.io/acme/app:new",
		CreatedAt:   time.Now().Add(-2 * time.Minute).UTC(),
		DeployedAt:  timePtr(time.Now().Add(-90 * time.Second).UTC()),
	})
	service := NewService(testLogger(), fakeStore, &fakePublisher{}, 100*time.Millisecond, 20*time.Millisecond)

	response, err := service.RollbackDeployment(context.Background(), "org-1", current.ID)
	if err != nil {
		t.Fatalf("rollback deployment failed: %v", err)
	}
	if response.Status != "rolled_back" {
		t.Fatalf("unexpected current status: got %s want %s", response.Status, "rolled_back")
	}
	if response.CurrentLiveDeployment != previous.ID {
		t.Fatalf("unexpected current live deployment: got %s want %s", response.CurrentLiveDeployment, previous.ID)
	}

	restoredPrevious, err := fakeStore.GetDeployment(context.Background(), previous.ID)
	if err != nil {
		t.Fatalf("get previous deployment failed: %v", err)
	}
	if restoredPrevious.Status != "live" {
		t.Fatalf("unexpected previous status: got %s want %s", restoredPrevious.Status, "live")
	}
	if restoredPrevious.ImageTag != previous.ImageTag {
		t.Fatalf("unexpected previous image tag: got %s want %s", restoredPrevious.ImageTag, previous.ImageTag)
	}
}

func TestDeploymentStatusAllTransitionsInDB(t *testing.T) {
	fakeStore := newFakeStore()
	service := NewService(testLogger(), fakeStore, &fakePublisher{}, 200*time.Millisecond, 25*time.Millisecond)

	response, err := service.StartDeployment(context.Background(), "org-1", StartRequest{
		RepoID:      "repo-1",
		CommitSHA:   "sha-1",
		Branch:      "main",
		AcceptRisk:  false,
		Environment: "production",
		ImageTag:    "ghcr.io/acme/app:1",
	})
	if err != nil {
		t.Fatalf("start deployment failed: %v", err)
	}

	storedDeployment, err := fakeStore.GetDeployment(context.Background(), response.ID)
	if err != nil {
		t.Fatalf("get deployment failed: %v", err)
	}
	if storedDeployment.Status != "deploying" {
		t.Fatalf("unexpected initial status: got %s want %s", storedDeployment.Status, "deploying")
	}

	assertEventually(t, time.Second, func() error {
		updatedDeployment, err := fakeStore.GetDeployment(context.Background(), response.ID)
		if err != nil {
			return err
		}
		if updatedDeployment.Status != "live" {
			return fmt.Errorf("status=%s", updatedDeployment.Status)
		}
		return nil
	})
}

func TestStartDeploymentBlocksRiskyImageWithoutAcceptRisk(t *testing.T) {
	service := NewService(testLogger(), newFakeStore(), &fakePublisher{}, 200*time.Millisecond, 25*time.Millisecond)

	_, err := service.StartDeployment(context.Background(), "org-1", StartRequest{
		RepoID:      "repo-1",
		CommitSHA:   "sha-risky",
		Branch:      "main",
		Environment: "production",
		ImageTag:    "ghcr.io/acme/app:risky",
		ScanResults: map[string]any{"critical": 1, "high": 3},
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestStartDeploymentAllowsRiskOverrideAndPersistsFlags(t *testing.T) {
	fakeStore := newFakeStore()
	service := NewService(testLogger(), fakeStore, &fakePublisher{}, 200*time.Millisecond, 25*time.Millisecond)

	response, err := service.StartDeployment(context.Background(), "org-1", StartRequest{
		RepoID:      "repo-1",
		CommitSHA:   "sha-risk-accepted",
		Branch:      "main",
		Environment: "production",
		ImageTag:    "ghcr.io/acme/app:risk-accepted",
		ScanResults: map[string]any{"critical": 1, "high": 2},
		AcceptRisk:  true,
	})
	if err != nil {
		t.Fatalf("start deployment failed: %v", err)
	}
	if !response.AcceptRisk {
		t.Fatal("expected accept_risk=true in response")
	}
	if response.ScanResults["critical"] != 1 {
		t.Fatalf("unexpected response scan_results: %+v", response.ScanResults)
	}

	storedDeployment, err := fakeStore.GetDeployment(context.Background(), response.ID)
	if err != nil {
		t.Fatalf("get deployment failed: %v", err)
	}
	if !storedDeployment.AcceptRisk {
		t.Fatal("expected accept_risk=true in persisted deployment")
	}
	if storedDeployment.ScanResults["critical"] != 1 {
		t.Fatalf("unexpected persisted scan_results: %+v", storedDeployment.ScanResults)
	}
}

type fakeStore struct {
	mu          sync.Mutex
	deployments map[string]store.Deployment
	sequence    int
	projectIDs  map[string]string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		deployments: make(map[string]store.Deployment),
		projectIDs:  map[string]string{"repo-1": "project-1"},
	}
}

func (s *fakeStore) CreateDeployment(_ context.Context, params store.CreateDeploymentParams) (store.Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	id := fmt.Sprintf("dep-%d", s.sequence)
	deployment := store.Deployment{
		ID:          id,
		RepoID:      params.RepoID,
		CommitSHA:   params.CommitSHA,
		Branch:      params.Branch,
		ScanResults: params.ScanResults,
		AcceptRisk:  params.AcceptRisk,
		Status:      params.Status,
		Environment: params.Environment,
		ImageTag:    params.ImageTag,
		CreatedAt:   time.Now().UTC(),
	}
	s.deployments[id] = deployment
	return deployment, nil
}

func (s *fakeStore) GetDeployment(_ context.Context, deploymentID string) (store.Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deployment, ok := s.deployments[deploymentID]
	if !ok {
		return store.Deployment{}, store.ErrNotFound
	}
	return deployment, nil
}

func (s *fakeStore) ListDeploymentsByProject(_ context.Context, projectID string, limit int) ([]store.Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 10
	}
	results := make([]store.Deployment, 0, limit)
	for _, deployment := range s.deployments {
		if repoProjectID, ok := s.projectIDs[deployment.RepoID]; !ok || repoProjectID != projectID {
			continue
		}
		results = append(results, deployment)
	}
	return results, nil
}

func (s *fakeStore) LookupProjectID(_ context.Context, repoID string) (string, error) {
	if projectID, ok := s.projectIDs[repoID]; ok {
		return projectID, nil
	}
	return "", store.ErrNotFound
}

func (s *fakeStore) FindActiveDeployment(_ context.Context, repoID, environment string) (*store.Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var active *store.Deployment
	for _, deployment := range s.deployments {
		if deployment.RepoID != repoID || deployment.Environment != environment || deployment.Status != "live" {
			continue
		}
		if active == nil || compareDeploymentFreshness(deployment, *active) > 0 {
			copy := deployment
			active = &copy
		}
	}
	return active, nil
}

func (s *fakeStore) FindPreviousDeployment(_ context.Context, repoID, environment, currentDeploymentID string) (*store.Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var previous *store.Deployment
	for _, deployment := range s.deployments {
		if deployment.ID == currentDeploymentID || deployment.RepoID != repoID || deployment.Environment != environment {
			continue
		}
		if deployment.Status != "live" && deployment.Status != "superseded" {
			continue
		}
		if previous == nil || compareDeploymentFreshness(deployment, *previous) > 0 {
			copy := deployment
			previous = &copy
		}
	}
	return previous, nil
}

func (s *fakeStore) MarkDeploymentFailed(_ context.Context, deploymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	deployment, ok := s.deployments[deploymentID]
	if !ok {
		return store.ErrNotFound
	}
	deployment.Status = "failed"
	s.deployments[deploymentID] = deployment
	return nil
}

func (s *fakeStore) PromoteDeployment(_ context.Context, deploymentID string, deployedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	deployment, ok := s.deployments[deploymentID]
	if !ok {
		return store.ErrNotFound
	}
	for id, candidate := range s.deployments {
		if id == deploymentID {
			continue
		}
		if candidate.RepoID == deployment.RepoID && candidate.Environment == deployment.Environment && candidate.Status == "live" {
			candidate.Status = "superseded"
			s.deployments[id] = candidate
		}
	}
	deployment.Status = "live"
	deployment.DeployedAt = timePtr(deployedAt.UTC())
	s.deployments[deploymentID] = deployment
	return nil
}

func (s *fakeStore) RollbackDeployment(_ context.Context, deploymentID string, rolledBackAt time.Time) (store.Deployment, store.Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.deployments[deploymentID]
	if !ok {
		return store.Deployment{}, store.Deployment{}, store.ErrNotFound
	}
	var previous *store.Deployment
	for id, candidate := range s.deployments {
		if id == deploymentID || candidate.RepoID != current.RepoID || candidate.Environment != current.Environment {
			continue
		}
		if candidate.Status != "live" && candidate.Status != "superseded" {
			continue
		}
		if previous == nil || compareDeploymentFreshness(candidate, *previous) > 0 {
			copy := candidate
			previous = &copy
		}
	}
	if previous == nil {
		return store.Deployment{}, store.Deployment{}, store.ErrNotFound
	}
	current.Status = "rolled_back"
	s.deployments[current.ID] = current
	prev := *previous
	prev.Status = "live"
	prev.DeployedAt = timePtr(rolledBackAt.UTC())
	s.deployments[prev.ID] = prev
	return current, prev, nil
}

func (s *fakeStore) seedDeployment(deployment store.Deployment) store.Deployment {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deployments[deployment.ID] = deployment
	return deployment
}

type fakePublisher struct {
	mu     sync.Mutex
	events []string
}

func (p *fakePublisher) Publish(_ context.Context, event any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	typedEvent, ok := event.(interface{ GetType() string })
	if ok {
		p.events = append(p.events, typedEvent.GetType())
		return nil
	}
	if payload, ok := event.(map[string]any); ok {
		if eventType, ok := payload["type"].(string); ok {
			p.events = append(p.events, eventType)
		}
		return nil
	}
	if envelope, ok := event.(struct {
		eventsdk.BaseEvent
		DeploymentID string `json:"deployment_id"`
		CommitSHA    string `json:"commit_sha"`
		Environment  string `json:"environment"`
		ImageTag     string `json:"image_tag"`
		Action       string `json:"action,omitempty"`
	}); ok {
		p.events = append(p.events, envelope.Type)
	}
	return nil
}

func (p *fakePublisher) Close() error { return nil }

func (p *fakePublisher) sawEvent(eventType string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, candidate := range p.events {
		if candidate == eventType {
			return true
		}
	}
	return false
}

func assertEventually(t *testing.T, timeout time.Duration, assertion func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := assertion(); err == nil {
			return
		} else {
			lastErr = err
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met before timeout: %v", lastErr)
}

func compareDeploymentFreshness(left, right store.Deployment) int {
	leftTime := left.CreatedAt
	if left.DeployedAt != nil {
		leftTime = *left.DeployedAt
	}
	rightTime := right.CreatedAt
	if right.DeployedAt != nil {
		rightTime = *right.DeployedAt
	}
	if leftTime.After(rightTime) {
		return 1
	}
	if rightTime.After(leftTime) {
		return -1
	}
	return 0
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func timePtr(value time.Time) *time.Time {
	return &value
}
