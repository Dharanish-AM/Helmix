package gitclone

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var pathSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// Clone performs a shallow clone of the repository into a deterministic temp directory.
func Clone(ctx context.Context, gitBinary, baseDir, repoURL, githubToken, repoID string) (string, error) {
	clonePath := filepath.Join(baseDir, sanitize(repoID))
	if err := os.RemoveAll(clonePath); err != nil {
		return "", fmt.Errorf("remove existing clone path: %w", err)
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", fmt.Errorf("create clone base dir: %w", err)
	}

	command := exec.CommandContext(ctx, gitBinary, "clone", "--depth=1", "--single-branch", authenticatedURL(repoURL, githubToken), clonePath)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return clonePath, nil
}

func authenticatedURL(repoURL, githubToken string) string {
	if strings.TrimSpace(githubToken) == "" {
		return repoURL
	}
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return repoURL
	}
	if parsedURL.Scheme != "https" {
		return repoURL
	}
	parsedURL.User = url.UserPassword("token", githubToken)
	return parsedURL.String()
}

func sanitize(value string) string {
	sanitized := pathSanitizer.ReplaceAllString(strings.TrimSpace(value), "-")
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		return "repo"
	}
	return sanitized
}
