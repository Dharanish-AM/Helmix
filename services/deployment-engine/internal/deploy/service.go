package deploy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	eventsdk "github.com/your-org/helmix/libs/event-sdk"
	"github.com/your-org/helmix/services/deployment-engine/internal/store"
)

var (
	ErrInvalidRequest      = errors.New("invalid deployment request")
	ErrRollbackUnavailable = errors.New("rollback target unavailable")
	ErrNotFound            = store.ErrNotFound
)

// StartRequest defines the deploy API input.
type StartRequest struct {
	RepoID            string `json:"repo_id"`
	CommitSHA         string `json:"commit_sha"`
	Branch            string `json:"branch"`
	Environment       string `json:"environment"`
	ImageTag          string `json:"image_tag"`
	ReadyAfterSeconds int    `json:"ready_after_seconds,omitempty"`
	SimulateFailure   bool   `json:"simulate_failure,omitempty"`
}

// DeploymentResponse is the API contract returned by deployment endpoints.
type DeploymentResponse struct {
	ID                    string     `json:"id"`
	RepoID                string     `json:"repo_id"`
	CommitSHA             string     `json:"commit_sha"`
	Branch                string     `json:"branch"`
	Status                string     `json:"status"`
	Environment           string     `json:"environment"`
	ImageTag              string     `json:"image_tag,omitempty"`
	DeployedAt            *time.Time `json:"deployed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	Active                bool       `json:"active"`
	CurrentLiveDeployment string     `json:"current_live_deployment_id,omitempty"`
	PreviousDeploymentID  string     `json:"previous_deployment_id,omitempty"`
}

// Store captures the persistence operations required by the deployment service.
type Store interface {
	CreateDeployment(ctx context.Context, params store.CreateDeploymentParams) (store.Deployment, error)
	GetDeployment(ctx context.Context, deploymentID string) (store.Deployment, error)
	LookupProjectID(ctx context.Context, repoID string) (string, error)
	FindActiveDeployment(ctx context.Context, repoID, environment string) (*store.Deployment, error)
	FindPreviousDeployment(ctx context.Context, repoID, environment, currentDeploymentID string) (*store.Deployment, error)
	MarkDeploymentFailed(ctx context.Context, deploymentID string) error
	PromoteDeployment(ctx context.Context, deploymentID string, deployedAt time.Time) error
	RollbackDeployment(ctx context.Context, deploymentID string, rolledBackAt time.Time) (store.Deployment, store.Deployment, error)
}

// Publisher emits deployment lifecycle events.
type Publisher interface {
	Publish(ctx context.Context, event any) error
	Close() error
}

// Service executes DB-backed deployment state transitions.
type Service struct {
	logger            *slog.Logger
	store             Store
	publisher         Publisher
	deployTimeout     time.Duration
	defaultReadyDelay time.Duration
}

// NewService constructs a deployment service.
func NewService(logger *slog.Logger, deploymentStore Store, publisher Publisher, deployTimeout, defaultReadyDelay time.Duration) *Service {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(nil, nil))
	}
	if publisher == nil {
		publisher = noopPublisher{}
	}
	return &Service{
		logger:            logger,
		store:             deploymentStore,
		publisher:         publisher,
		deployTimeout:     deployTimeout,
		defaultReadyDelay: defaultReadyDelay,
	}
}

// StartDeployment creates and asynchronously promotes a deployment.
func (s *Service) StartDeployment(ctx context.Context, orgID string, request StartRequest) (DeploymentResponse, error) {
	if err := validateStartRequest(request); err != nil {
		return DeploymentResponse{}, err
	}

	deployment, err := s.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		RepoID:      strings.TrimSpace(request.RepoID),
		CommitSHA:   strings.TrimSpace(request.CommitSHA),
		Branch:      strings.TrimSpace(request.Branch),
		Environment: strings.TrimSpace(request.Environment),
		ImageTag:    strings.TrimSpace(request.ImageTag),
		Status:      "deploying",
	})
	if err != nil {
		return DeploymentResponse{}, fmt.Errorf("create deployment: %w", err)
	}

	projectID, err := s.store.LookupProjectID(ctx, deployment.RepoID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return DeploymentResponse{}, fmt.Errorf("lookup project for deployment: %w", err)
	}

	if err := s.publishLifecycleEvent(ctx, eventsdk.DeploymentStarted, orgID, projectID, deployment, ""); err != nil {
		s.logger.Warn("publish deployment.started failed", slog.String("deployment_id", deployment.ID), slog.String("error", err.Error()))
	}

	readyDelay := s.defaultReadyDelay
	if request.ReadyAfterSeconds > 0 {
		readyDelay = time.Duration(request.ReadyAfterSeconds) * time.Second
	}

	go s.completeDeployment(deployment, orgID, projectID, readyDelay, request.SimulateFailure)

	return s.buildResponse(ctx, deployment)
}

// GetDeployment returns the current deployment state with active/previous context.
func (s *Service) GetDeployment(ctx context.Context, deploymentID string) (DeploymentResponse, error) {
	deployment, err := s.store.GetDeployment(ctx, strings.TrimSpace(deploymentID))
	if err != nil {
		return DeploymentResponse{}, err
	}
	return s.buildResponse(ctx, deployment)
}

// RollbackDeployment reactivates the previous deployment and marks the current one rolled back.
func (s *Service) RollbackDeployment(ctx context.Context, orgID, deploymentID string) (DeploymentResponse, error) {
	deploymentID = strings.TrimSpace(deploymentID)
	if deploymentID == "" {
		return DeploymentResponse{}, fmt.Errorf("%w: deployment id is required", ErrInvalidRequest)
	}

	current, previous, err := s.store.RollbackDeployment(ctx, deploymentID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return DeploymentResponse{}, ErrRollbackUnavailable
		}
		return DeploymentResponse{}, fmt.Errorf("rollback deployment: %w", err)
	}

	projectID, err := s.store.LookupProjectID(ctx, current.RepoID)
	if err == nil {
		if publishErr := s.publishLifecycleEvent(ctx, eventsdk.DeploymentFailed, orgID, projectID, current, "rollback"); publishErr != nil {
			s.logger.Warn("publish deployment.failed rollback failed", slog.String("deployment_id", current.ID), slog.String("error", publishErr.Error()))
		}
		_ = previous
	}

	return s.buildResponse(ctx, current)
}

func (s *Service) buildResponse(ctx context.Context, deployment store.Deployment) (DeploymentResponse, error) {
	activeDeployment, err := s.store.FindActiveDeployment(ctx, deployment.RepoID, deployment.Environment)
	if err != nil {
		return DeploymentResponse{}, fmt.Errorf("find active deployment: %w", err)
	}
	previousDeployment, err := s.store.FindPreviousDeployment(ctx, deployment.RepoID, deployment.Environment, deployment.ID)
	if err != nil {
		return DeploymentResponse{}, fmt.Errorf("find previous deployment: %w", err)
	}

	response := DeploymentResponse{
		ID:          deployment.ID,
		RepoID:      deployment.RepoID,
		CommitSHA:   deployment.CommitSHA,
		Branch:      deployment.Branch,
		Status:      deployment.Status,
		Environment: deployment.Environment,
		ImageTag:    deployment.ImageTag,
		DeployedAt:  deployment.DeployedAt,
		CreatedAt:   deployment.CreatedAt,
	}
	if activeDeployment != nil {
		response.CurrentLiveDeployment = activeDeployment.ID
		response.Active = activeDeployment.ID == deployment.ID
	}
	if previousDeployment != nil {
		response.PreviousDeploymentID = previousDeployment.ID
	}
	return response, nil
}

func (s *Service) completeDeployment(deployment store.Deployment, orgID, projectID string, readyDelay time.Duration, simulateFailure bool) {
	ctx, cancel := context.WithTimeout(context.Background(), s.deployTimeout)
	defer cancel()

	timer := time.NewTimer(readyDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		s.failDeployment(context.Background(), orgID, projectID, deployment)
		return
	case <-timer.C:
	}

	if simulateFailure {
		s.failDeployment(context.Background(), orgID, projectID, deployment)
		return
	}

	promotedAt := time.Now().UTC()
	if err := s.store.PromoteDeployment(context.Background(), deployment.ID, promotedAt); err != nil {
		s.logger.Error("promote deployment failed", slog.String("deployment_id", deployment.ID), slog.String("error", err.Error()))
		s.failDeployment(context.Background(), orgID, projectID, deployment)
		return
	}

	updatedDeployment, err := s.store.GetDeployment(context.Background(), deployment.ID)
	if err != nil {
		s.logger.Error("reload promoted deployment failed", slog.String("deployment_id", deployment.ID), slog.String("error", err.Error()))
		return
	}

	if err := s.publishLifecycleEvent(context.Background(), eventsdk.DeploymentSucceeded, orgID, projectID, updatedDeployment, ""); err != nil {
		s.logger.Warn("publish deployment.succeeded failed", slog.String("deployment_id", deployment.ID), slog.String("error", err.Error()))
	}
	_ = updatedDeployment
}

func (s *Service) failDeployment(ctx context.Context, orgID, projectID string, deployment store.Deployment) {
	if err := s.store.MarkDeploymentFailed(ctx, deployment.ID); err != nil {
		s.logger.Error("mark deployment failed failed", slog.String("deployment_id", deployment.ID), slog.String("error", err.Error()))
		return
	}
	updatedDeployment, err := s.store.GetDeployment(ctx, deployment.ID)
	if err != nil {
		updatedDeployment = deployment
		updatedDeployment.Status = "failed"
	}
	if err := s.publishLifecycleEvent(ctx, eventsdk.DeploymentFailed, orgID, projectID, updatedDeployment, "timeout"); err != nil {
		s.logger.Warn("publish deployment.failed failed", slog.String("deployment_id", deployment.ID), slog.String("error", err.Error()))
	}
}

func (s *Service) publishLifecycleEvent(ctx context.Context, eventType eventsdk.EventType, orgID, projectID string, deployment store.Deployment, action string) error {
	event := struct {
		eventsdk.BaseEvent
		DeploymentID string `json:"deployment_id"`
		CommitSHA    string `json:"commit_sha"`
		Environment  string `json:"environment"`
		ImageTag     string `json:"image_tag"`
		Action       string `json:"action,omitempty"`
	}{
		BaseEvent: eventsdk.BaseEvent{
			ID:        deployment.ID,
			Type:      string(eventType),
			OrgID:     orgID,
			ProjectID: projectID,
			CreatedAt: time.Now().UTC(),
		},
		DeploymentID: deployment.ID,
		CommitSHA:    deployment.CommitSHA,
		Environment:  deployment.Environment,
		ImageTag:     deployment.ImageTag,
		Action:       action,
	}
	return s.publisher.Publish(ctx, event)
}

func validateStartRequest(request StartRequest) error {
	if strings.TrimSpace(request.RepoID) == "" {
		return fmt.Errorf("%w: repo_id is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(request.CommitSHA) == "" {
		return fmt.Errorf("%w: commit_sha is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(request.Branch) == "" {
		return fmt.Errorf("%w: branch is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(request.Environment) == "" {
		return fmt.Errorf("%w: environment is required", ErrInvalidRequest)
	}
	if request.ReadyAfterSeconds < 0 {
		return fmt.Errorf("%w: ready_after_seconds must be zero or positive", ErrInvalidRequest)
	}
	return nil
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, any) error { return nil }

func (noopPublisher) Close() error { return nil }