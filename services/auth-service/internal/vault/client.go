package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound    = errors.New("secret not found")
	ErrUnavailable = errors.New("vault unavailable")
)

var validPathSegment = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type Config struct {
	Address         string
	AppRoleID       string
	AppRoleSecretID string
	KVMount         string
	TokenExpirySkew time.Duration
}

type SecretRecord struct {
	Service string `json:"service"`
	Key     string `json:"key"`
	Value   any    `json:"value"`
	Version int    `json:"version"`
}

type SecretClient interface {
	UpsertSecret(ctx context.Context, service, key string, value any) (SecretRecord, error)
	GetSecret(ctx context.Context, service, key string) (SecretRecord, error)
	DeleteSecret(ctx context.Context, service, key string) error
}

type HTTPClient struct {
	httpClient *http.Client
	address    *url.URL
	roleID     string
	secretID   string
	kvMount    string
	skew       time.Duration

	mu          sync.Mutex
	clientToken string
	tokenExpiry time.Time
}

func NewHTTPClient(cfg Config, httpClient *http.Client) (*HTTPClient, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, fmt.Errorf("vault address is required")
	}
	if strings.TrimSpace(cfg.AppRoleID) == "" {
		return nil, fmt.Errorf("vault approle role id is required")
	}
	if strings.TrimSpace(cfg.AppRoleSecretID) == "" {
		return nil, fmt.Errorf("vault approle secret id is required")
	}
	if strings.TrimSpace(cfg.KVMount) == "" {
		return nil, fmt.Errorf("vault kv mount is required")
	}
	parsedAddress, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("parse vault address: %w", err)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	skew := cfg.TokenExpirySkew
	if skew <= 0 {
		skew = 15 * time.Second
	}

	return &HTTPClient{
		httpClient: httpClient,
		address:    parsedAddress,
		roleID:     cfg.AppRoleID,
		secretID:   cfg.AppRoleSecretID,
		kvMount:    strings.Trim(cfg.KVMount, "/"),
		skew:       skew,
	}, nil
}

func (c *HTTPClient) UpsertSecret(ctx context.Context, service, key string, value any) (SecretRecord, error) {
	service, key, err := sanitizeSecretPath(service, key)
	if err != nil {
		return SecretRecord{}, err
	}

	body, err := json.Marshal(map[string]any{"data": map[string]any{"value": value}})
	if err != nil {
		return SecretRecord{}, fmt.Errorf("marshal vault upsert payload: %w", err)
	}

	statusCode, _, err := c.sendKVRequest(ctx, http.MethodPost, c.dataPath(service, key), body)
	if err != nil {
		return SecretRecord{}, err
	}
	if statusCode >= http.StatusBadRequest {
		return SecretRecord{}, classifyVaultStatus(statusCode)
	}

	readRecord, err := c.GetSecret(ctx, service, key)
	if err != nil {
		return SecretRecord{}, err
	}
	return readRecord, nil
}

func (c *HTTPClient) GetSecret(ctx context.Context, service, key string) (SecretRecord, error) {
	service, key, err := sanitizeSecretPath(service, key)
	if err != nil {
		return SecretRecord{}, err
	}

	statusCode, responseBody, err := c.sendKVRequest(ctx, http.MethodGet, c.dataPath(service, key), nil)
	if err != nil {
		return SecretRecord{}, err
	}
	if statusCode == http.StatusNotFound {
		return SecretRecord{}, ErrNotFound
	}
	if statusCode >= http.StatusBadRequest {
		return SecretRecord{}, classifyVaultStatus(statusCode)
	}

	var response struct {
		Data struct {
			Data map[string]any `json:"data"`
			Meta struct {
				Version int `json:"version"`
			} `json:"metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return SecretRecord{}, fmt.Errorf("decode vault get response: %w", err)
	}

	value, ok := response.Data.Data["value"]
	if !ok {
		return SecretRecord{}, fmt.Errorf("vault response missing value field")
	}

	return SecretRecord{
		Service: service,
		Key:     key,
		Value:   value,
		Version: response.Data.Meta.Version,
	}, nil
}

func (c *HTTPClient) DeleteSecret(ctx context.Context, service, key string) error {
	service, key, err := sanitizeSecretPath(service, key)
	if err != nil {
		return err
	}

	statusCode, _, err := c.sendKVRequest(ctx, http.MethodDelete, c.dataPath(service, key), nil)
	if err != nil {
		return err
	}
	if statusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if statusCode >= http.StatusBadRequest {
		return classifyVaultStatus(statusCode)
	}
	return nil
}

func sanitizeSecretPath(service, key string) (string, string, error) {
	trimmedService := strings.TrimSpace(service)
	trimmedKey := strings.TrimSpace(key)
	if trimmedService == "" || trimmedKey == "" {
		return "", "", fmt.Errorf("service and key are required")
	}
	if !validPathSegment.MatchString(trimmedService) || !validPathSegment.MatchString(trimmedKey) {
		return "", "", fmt.Errorf("service and key may only contain letters, numbers, dashes, and underscores")
	}
	return trimmedService, trimmedKey, nil
}

func (c *HTTPClient) dataPath(service, key string) string {
	return fmt.Sprintf("/v1/%s/data/%s/%s", c.kvMount, url.PathEscape(service), url.PathEscape(key))
}

func (c *HTTPClient) sendKVRequest(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	token, err := c.ensureClientToken(ctx)
	if err != nil {
		return 0, nil, err
	}

	requestURL := *c.address
	requestURL.Path = path

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		requestBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), requestBody)
	if err != nil {
		return 0, nil, fmt.Errorf("build vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("vault request failed: %w", ErrUnavailable)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		// Token may have expired unexpectedly; retry once with a fresh login.
		if err := c.forceRelogin(ctx); err == nil {
			return c.sendKVRequestWithoutRetry(ctx, method, path, body)
		}
	}

	return resp.StatusCode, responseBody, nil
}

func (c *HTTPClient) sendKVRequestWithoutRetry(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	token, err := c.ensureClientToken(ctx)
	if err != nil {
		return 0, nil, err
	}
	requestURL := *c.address
	requestURL.Path = path

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		requestBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), requestBody)
	if err != nil {
		return 0, nil, fmt.Errorf("build vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("vault request failed: %w", ErrUnavailable)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, responseBody, nil
}

func (c *HTTPClient) ensureClientToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.clientToken != "" && time.Now().Add(c.skew).Before(c.tokenExpiry) {
		token := c.clientToken
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	if err := c.forceRelogin(ctx); err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.clientToken == "" {
		return "", fmt.Errorf("vault login did not produce client token")
	}
	return c.clientToken, nil
}

func (c *HTTPClient) forceRelogin(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	loginPayload, err := json.Marshal(map[string]string{
		"role_id":   c.roleID,
		"secret_id": c.secretID,
	})
	if err != nil {
		return fmt.Errorf("marshal vault login payload: %w", err)
	}

	loginURL := *c.address
	loginURL.Path = "/v1/auth/approle/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL.String(), bytes.NewReader(loginPayload))
	if err != nil {
		return fmt.Errorf("build vault login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault login failed: %w", ErrUnavailable)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("vault login failed: %w", classifyVaultStatus(resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read vault login response: %w", err)
	}

	var loginResponse struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}
	if err := json.Unmarshal(body, &loginResponse); err != nil {
		return fmt.Errorf("decode vault login response: %w", err)
	}
	if strings.TrimSpace(loginResponse.Auth.ClientToken) == "" {
		return fmt.Errorf("vault login response missing client token")
	}

	leaseDuration := loginResponse.Auth.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = 300
	}

	c.clientToken = loginResponse.Auth.ClientToken
	c.tokenExpiry = time.Now().Add(time.Duration(leaseDuration) * time.Second)
	return nil
}

func classifyVaultStatus(statusCode int) error {
	if statusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if statusCode >= http.StatusInternalServerError {
		return ErrUnavailable
	}
	if statusCode == http.StatusForbidden || statusCode == http.StatusUnauthorized {
		return fmt.Errorf("vault access denied (status=%d)", statusCode)
	}
	return fmt.Errorf("vault request failed with status %d", statusCode)
}

func (s SecretRecord) String() string {
	return s.Service + "/" + s.Key + "@v" + strconv.Itoa(s.Version)
}
