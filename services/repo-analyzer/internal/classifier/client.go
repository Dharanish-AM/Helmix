package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	eventsdk "github.com/your-org/helmix/libs/event-sdk"

	"github.com/your-org/helmix/services/repo-analyzer/internal/analyzer"
)

type classifyRequest struct {
	Files []analyzer.SampleFile `json:"files"`
}

type classifyResponse struct {
	Runtime       string   `json:"runtime"`
	Framework     string   `json:"framework"`
	Database      []string `json:"database"`
	Containerized bool     `json:"containerized"`
	Port          int      `json:"port"`
	BuildCommand  string   `json:"build_command"`
	TestCommand   string   `json:"test_command"`
	Confidence    float64  `json:"confidence"`
}

// Client calls the optional incident-ai fallback classifier.
type Client struct {
	endpoint   string
	httpClient *http.Client
}

// New returns a fallback classifier client.
func New(endpoint string, timeout time.Duration) *Client {
	return &Client{endpoint: strings.TrimSpace(endpoint), httpClient: &http.Client{Timeout: timeout}}
}

// Enrich attempts to fill an unknown stack using incident-ai classification.
func (c *Client) Enrich(ctx context.Context, result analyzer.Result) (analyzer.Result, error) {
	if c == nil || c.endpoint == "" || len(result.SampledFiles) == 0 {
		return result, nil
	}
	payload, err := json.Marshal(classifyRequest{Files: result.SampledFiles})
	if err != nil {
		return result, fmt.Errorf("marshal classify request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return result, fmt.Errorf("create classify request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return result, fmt.Errorf("call classify endpoint: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return result, fmt.Errorf("classify endpoint returned %d", response.StatusCode)
	}

	var classified classifyResponse
	if err := json.NewDecoder(response.Body).Decode(&classified); err != nil {
		return result, fmt.Errorf("decode classify response: %w", err)
	}
	if strings.TrimSpace(classified.Runtime) == "" || strings.TrimSpace(classified.Framework) == "" {
		return result, nil
	}

	result.Stack = eventsdk.DetectedStack{
		Runtime:       classified.Runtime,
		Framework:     classified.Framework,
		Database:      classified.Database,
		Containerized: classified.Containerized,
		HasTests:      result.Stack.HasTests,
		Port:          classified.Port,
		BuildCommand:  classified.BuildCommand,
		TestCommand:   classified.TestCommand,
	}
	result.Confidence = classified.Confidence
	result.FallbackUsed = true
	return result, nil
}
