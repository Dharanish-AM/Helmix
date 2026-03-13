package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectNextJS(t *testing.T) {
	repoDir := t.TempDir()
	writeFile(t, repoDir, "package.json", `{"dependencies":{"next":"14.0.0","react":"18.0.0"},"engines":{"node":">=18"}}`)

	result, err := DetectRepository(repoDir)
	if err != nil {
		t.Fatalf("detect repository: %v", err)
	}
	if result.Stack.Runtime != "node" || result.Stack.Framework != "nextjs" {
		t.Fatalf("expected node/nextjs, got %s/%s", result.Stack.Runtime, result.Stack.Framework)
	}
}

func TestDetectDjango(t *testing.T) {
	repoDir := t.TempDir()
	writeFile(t, repoDir, "requirements.txt", "Django==5.0.3\n")

	result, err := DetectRepository(repoDir)
	if err != nil {
		t.Fatalf("detect repository: %v", err)
	}
	if result.Stack.Runtime != "python" || result.Stack.Framework != "django" {
		t.Fatalf("expected python/django, got %s/%s", result.Stack.Runtime, result.Stack.Framework)
	}
}

func TestDetectSpringBoot(t *testing.T) {
	repoDir := t.TempDir()
	writeFile(t, repoDir, "pom.xml", `<project><artifactId>demo</artifactId><dependency>spring-boot-starter-web</dependency></project>`)

	result, err := DetectRepository(repoDir)
	if err != nil {
		t.Fatalf("detect repository: %v", err)
	}
	if result.Stack.Runtime != "java" || result.Stack.Framework != "spring" {
		t.Fatalf("expected java/spring, got %s/%s", result.Stack.Runtime, result.Stack.Framework)
	}
}

func TestDetectGin(t *testing.T) {
	repoDir := t.TempDir()
	writeFile(t, repoDir, "go.mod", "module example.com/demo\n\nrequire github.com/gin-gonic/gin v1.10.0\n")

	result, err := DetectRepository(repoDir)
	if err != nil {
		t.Fatalf("detect repository: %v", err)
	}
	if result.Stack.Runtime != "go" || result.Stack.Framework != "gin" {
		t.Fatalf("expected go/gin, got %s/%s", result.Stack.Runtime, result.Stack.Framework)
	}
}

func TestDetectContainerized(t *testing.T) {
	repoDir := t.TempDir()
	writeFile(t, repoDir, "Dockerfile", "FROM node:18-alpine\n")

	result, err := DetectRepository(repoDir)
	if err != nil {
		t.Fatalf("detect repository: %v", err)
	}
	if !result.Stack.Containerized {
		t.Fatal("expected containerized=true")
	}
}

func TestDetectPostgresFromEnv(t *testing.T) {
	repoDir := t.TempDir()
	writeFile(t, repoDir, ".env.example", "DATABASE_URL=postgres://localhost:5432/app\n")

	result, err := DetectRepository(repoDir)
	if err != nil {
		t.Fatalf("detect repository: %v", err)
	}
	if len(result.Stack.Database) != 1 || result.Stack.Database[0] != "postgres" {
		t.Fatalf("expected postgres database hint, got %#v", result.Stack.Database)
	}
}

func TestUnknownStack(t *testing.T) {
	repoDir := t.TempDir()
	writeFile(t, repoDir, "README.md", "hello\n")

	result, err := DetectRepository(repoDir)
	if err != nil {
		t.Fatalf("detect repository: %v", err)
	}
	if result.Confidence >= 0.70 {
		t.Fatalf("expected confidence < 0.70, got %.2f", result.Confidence)
	}
	if !result.FallbackUsed {
		t.Fatal("expected fallback flag to be true for unknown stack")
	}
}

func writeFile(t *testing.T, root, relativePath, contents string) {
	t.Helper()
	fullPath := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", relativePath, err)
	}
}
