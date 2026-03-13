package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// User is the subset of GitHub profile data needed by Helmix auth.
type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// Repository is the subset of GitHub repository data needed by the dashboard picker.
type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	UpdatedAt     string `json:"updated_at"`
}

type emailAddress struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// Client exchanges OAuth codes and loads GitHub user profile data.
type Client struct {
	httpClient   *http.Client
	oauthBaseURL string
	apiBaseURL   string
	clientID     string
	clientSecret string
	redirectURL  string
}

// New constructs a GitHub OAuth/API client.
func New(httpClient *http.Client, oauthBaseURL, apiBaseURL, clientID, clientSecret, redirectURL string) *Client {
	return &Client{
		httpClient:   httpClient,
		oauthBaseURL: strings.TrimRight(oauthBaseURL, "/"),
		apiBaseURL:   strings.TrimRight(apiBaseURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
	}
}

// ExchangeCode exchanges an OAuth code for a GitHub access token.
func (c *Client) ExchangeCode(ctx context.Context, code string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
		"code":          code,
		"redirect_uri":  c.redirectURL,
	})
	if err != nil {
		return "", fmt.Errorf("marshal exchange request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.oauthBaseURL+"/access_token", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create exchange request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}
	defer response.Body.Close()

	var payload struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode exchange response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("exchange failed with status %d: %s", response.StatusCode, payload.Error)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("exchange returned empty access token")
	}

	return payload.AccessToken, nil
}

// FetchUser loads the GitHub profile and primary email for the access token.
func (c *Client) FetchUser(ctx context.Context, accessToken string) (User, error) {
	userRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL("user"), nil)
	if err != nil {
		return User{}, fmt.Errorf("create user request: %w", err)
	}
	userRequest.Header.Set("Accept", "application/vnd.github+json")
	userRequest.Header.Set("Authorization", "Bearer "+accessToken)
	userRequest.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	userResponse, err := c.httpClient.Do(userRequest)
	if err != nil {
		return User{}, fmt.Errorf("fetch user: %w", err)
	}
	defer userResponse.Body.Close()

	var user User
	if err := json.NewDecoder(userResponse.Body).Decode(&user); err != nil {
		return User{}, fmt.Errorf("decode user response: %w", err)
	}
	if userResponse.StatusCode >= http.StatusBadRequest {
		return User{}, fmt.Errorf("user request failed with status %d", userResponse.StatusCode)
	}
	if user.Email != "" {
		return user, nil
	}

	emailsRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL("user/emails"), nil)
	if err != nil {
		return User{}, fmt.Errorf("create emails request: %w", err)
	}
	emailsRequest.Header.Set("Accept", "application/vnd.github+json")
	emailsRequest.Header.Set("Authorization", "Bearer "+accessToken)
	emailsRequest.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	emailsResponse, err := c.httpClient.Do(emailsRequest)
	if err != nil {
		return User{}, fmt.Errorf("fetch emails: %w", err)
	}
	defer emailsResponse.Body.Close()

	var emails []emailAddress
	if err := json.NewDecoder(emailsResponse.Body).Decode(&emails); err != nil {
		return User{}, fmt.Errorf("decode emails response: %w", err)
	}
	if emailsResponse.StatusCode >= http.StatusBadRequest {
		return User{}, fmt.Errorf("emails request failed with status %d", emailsResponse.StatusCode)
	}
	for _, email := range emails {
		if email.Primary && email.Verified {
			user.Email = email.Email
			break
		}
	}
	if user.Email == "" && len(emails) > 0 {
		user.Email = emails[0].Email
	}

	return user, nil
}

// AuthorizeURL returns the GitHub OAuth redirect URL.
func (c *Client) AuthorizeURL(state string) string {
	values := url.Values{}
	values.Set("client_id", c.clientID)
	values.Set("redirect_uri", c.redirectURL)
	values.Set("scope", "read:user user:email repo")
	values.Set("state", state)
	return c.oauthBaseURL + "/authorize?" + values.Encode()
}

// ListRepositories loads repositories the authenticated user can access.
func (c *Client) ListRepositories(ctx context.Context, accessToken string, limit int) ([]Repository, error) {
	if limit <= 0 {
		limit = 50
	}

	reposURL, err := url.Parse(c.apiURL("user/repos"))
	if err != nil {
		return nil, fmt.Errorf("parse repos url: %w", err)
	}
	query := reposURL.Query()
	query.Set("sort", "updated")
	query.Set("per_page", fmt.Sprintf("%d", limit))
	query.Set("affiliation", "owner,collaborator,organization_member")
	reposURL.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, reposURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create repos request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch repositories: %w", err)
	}
	defer response.Body.Close()

	var repos []Repository
	if err := json.NewDecoder(response.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("decode repositories response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("repositories request failed with status %d", response.StatusCode)
	}

	return repos, nil
}

func (c *Client) apiURL(relativePath string) string {
	baseURL, _ := url.Parse(c.apiBaseURL)
	baseURL.Path = path.Join(baseURL.Path, relativePath)
	return baseURL.String()
}
