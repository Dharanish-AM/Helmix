package analyzer

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	eventsdk "github.com/your-org/helmix/libs/event-sdk"
)

// SampleFile contains a truncated file sample used for fallback classification.
type SampleFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Result contains the detected stack plus analyzer metadata.
type Result struct {
	Stack        eventsdk.DetectedStack `json:"stack"`
	Confidence   float64                `json:"confidence"`
	FallbackUsed bool                   `json:"fallback_used"`
	SampledFiles []SampleFile           `json:"sampled_files,omitempty"`
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Engines         struct {
		Node string `json:"node"`
	} `json:"engines"`
}

// DetectRepository analyzes a cloned repository and returns the detected stack.
func DetectRepository(root string) (Result, error) {
	result := Result{
		Stack: eventsdk.DetectedStack{
			Database: []string{},
			Port:     8080,
		},
		Confidence: 0.25,
	}

	packageJSONPath := filepath.Join(root, "package.json")
	if fileExists(packageJSONPath) {
		packageResult, err := detectNode(packageJSONPath)
		if err != nil {
			return Result{}, fmt.Errorf("detect node stack: %w", err)
		}
		if packageResult.Stack.Runtime != "" {
			result = packageResult
		}
	}

	if result.Stack.Runtime == "" {
		pythonResult, err := detectPython(root)
		if err != nil {
			return Result{}, fmt.Errorf("detect python stack: %w", err)
		}
		if pythonResult.Stack.Runtime != "" {
			result = pythonResult
		}
	}

	if result.Stack.Runtime == "" {
		javaResult, err := detectJava(filepath.Join(root, "pom.xml"))
		if err != nil {
			return Result{}, fmt.Errorf("detect java stack: %w", err)
		}
		if javaResult.Stack.Runtime != "" {
			result = javaResult
		}
	}

	if result.Stack.Runtime == "" {
		goResult, err := detectGo(filepath.Join(root, "go.mod"))
		if err != nil {
			return Result{}, fmt.Errorf("detect go stack: %w", err)
		}
		if goResult.Stack.Runtime != "" {
			result = goResult
		}
	}

	if result.Stack.Runtime == "" {
		rubyResult, err := detectRuby(filepath.Join(root, "Gemfile"))
		if err != nil {
			return Result{}, fmt.Errorf("detect ruby stack: %w", err)
		}
		if rubyResult.Stack.Runtime != "" {
			result = rubyResult
		}
	}

	if fileExists(filepath.Join(root, "Dockerfile")) {
		result.Stack.Containerized = true
	}

	databaseHints := make(map[string]struct{})
	if err := collectDatabaseHints(filepath.Join(root, "docker-compose.yml"), databaseHints); err != nil {
		return Result{}, fmt.Errorf("detect docker-compose database hints: %w", err)
	}
	if err := collectDatabaseHints(filepath.Join(root, ".env.example"), databaseHints); err != nil {
		return Result{}, fmt.Errorf("detect env database hints: %w", err)
	}
	result.Stack.Database = sortedKeys(databaseHints)
	result.Stack.HasTests = hasTests(root)
	applyDefaultCommands(&result.Stack)

	if result.Stack.Runtime == "" || result.Stack.Framework == "" {
		result.Confidence = 0.25
		result.FallbackUsed = true
		result.SampledFiles = SampleRepository(root)
	}

	return result, nil
}

// SampleRepository returns a stable, truncated set of file samples for fallback classification.
func SampleRepository(root string) []SampleFile {
	candidates := []string{
		"package.json",
		"requirements.txt",
		"pyproject.toml",
		"pom.xml",
		"go.mod",
		"Gemfile",
		"Dockerfile",
		"docker-compose.yml",
		".env.example",
	}

	samples := make([]SampleFile, 0, len(candidates))
	for _, relativePath := range candidates {
		fullPath := filepath.Join(root, relativePath)
		if !fileExists(fullPath) {
			continue
		}
		contents, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		content := string(contents)
		if len(content) > 2048 {
			content = content[:2048]
		}
		samples = append(samples, SampleFile{Path: relativePath, Content: content})
	}
	return samples
}

func detectNode(packageJSONPath string) (Result, error) {
	contents, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return Result{}, fmt.Errorf("read package.json: %w", err)
	}

	var manifest packageJSON
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return Result{}, fmt.Errorf("unmarshal package.json: %w", err)
	}

	dependencies := mergedDependencies(manifest.Dependencies, manifest.DevDependencies)
	result := Result{Stack: eventsdk.DetectedStack{Runtime: "node", Database: []string{}, Port: 3000}, Confidence: 0.80}
	switch {
	case hasDependency(dependencies, "next"):
		result.Stack.Framework = "nextjs"
		result.Confidence = 0.95
	case hasDependency(dependencies, "react"):
		result.Stack.Framework = "react"
		result.Confidence = 0.90
	case hasDependency(dependencies, "express"):
		result.Stack.Framework = "express"
		result.Confidence = 0.88
	case hasDependency(dependencies, "@nestjs/core"):
		result.Stack.Framework = "nestjs"
		result.Confidence = 0.90
	default:
		result.Stack = eventsdk.DetectedStack{Database: []string{}, Port: 8080}
		result.Confidence = 0.25
	}

	_ = strings.TrimSpace(manifest.Engines.Node)
	applyDefaultCommands(&result.Stack)
	return result, nil
}

func detectPython(root string) (Result, error) {
	for _, filename := range []string{"requirements.txt", "pyproject.toml"} {
		fullPath := filepath.Join(root, filename)
		if !fileExists(fullPath) {
			continue
		}
		contents, err := os.ReadFile(fullPath)
		if err != nil {
			return Result{}, fmt.Errorf("read %s: %w", filename, err)
		}
		lower := strings.ToLower(string(contents))
		result := Result{Stack: eventsdk.DetectedStack{Runtime: "python", Database: []string{}, Port: 8000}, Confidence: 0.90}
		switch {
		case strings.Contains(lower, "django"):
			result.Stack.Framework = "django"
		case strings.Contains(lower, "fastapi"):
			result.Stack.Framework = "fastapi"
		case strings.Contains(lower, "flask"):
			result.Stack.Framework = "flask"
		default:
			continue
		}
		applyDefaultCommands(&result.Stack)
		return result, nil
	}

	return Result{Stack: eventsdk.DetectedStack{Database: []string{}, Port: 8080}, Confidence: 0.25}, nil
}

func detectJava(pomPath string) (Result, error) {
	if !fileExists(pomPath) {
		return Result{Stack: eventsdk.DetectedStack{Database: []string{}, Port: 8080}, Confidence: 0.25}, nil
	}
	contents, err := os.ReadFile(pomPath)
	if err != nil {
		return Result{}, fmt.Errorf("read pom.xml: %w", err)
	}
	if !strings.Contains(strings.ToLower(string(contents)), "spring-boot") {
		return Result{Stack: eventsdk.DetectedStack{Database: []string{}, Port: 8080}, Confidence: 0.25}, nil
	}
	result := Result{Stack: eventsdk.DetectedStack{Runtime: "java", Framework: "spring", Database: []string{}, Port: 8080}, Confidence: 0.90}
	applyDefaultCommands(&result.Stack)
	return result, nil
}

func detectGo(goModPath string) (Result, error) {
	if !fileExists(goModPath) {
		return Result{Stack: eventsdk.DetectedStack{Database: []string{}, Port: 8080}, Confidence: 0.25}, nil
	}
	contents, err := os.ReadFile(goModPath)
	if err != nil {
		return Result{}, fmt.Errorf("read go.mod: %w", err)
	}
	lower := strings.ToLower(string(contents))
	result := Result{Stack: eventsdk.DetectedStack{Runtime: "go", Database: []string{}, Port: 8080}, Confidence: 0.90}
	switch {
	case strings.Contains(lower, "gin-gonic/gin"):
		result.Stack.Framework = "gin"
	case strings.Contains(lower, "labstack/echo"):
		result.Stack.Framework = "echo"
	case strings.Contains(lower, "gofiber/fiber"):
		result.Stack.Framework = "fiber"
	default:
		return Result{Stack: eventsdk.DetectedStack{Database: []string{}, Port: 8080}, Confidence: 0.25}, nil
	}
	applyDefaultCommands(&result.Stack)
	return result, nil
}

func detectRuby(gemfilePath string) (Result, error) {
	if !fileExists(gemfilePath) {
		return Result{Stack: eventsdk.DetectedStack{Database: []string{}, Port: 8080}, Confidence: 0.25}, nil
	}
	contents, err := os.ReadFile(gemfilePath)
	if err != nil {
		return Result{}, fmt.Errorf("read Gemfile: %w", err)
	}
	if !strings.Contains(strings.ToLower(string(contents)), "rails") {
		return Result{Stack: eventsdk.DetectedStack{Database: []string{}, Port: 8080}, Confidence: 0.25}, nil
	}
	result := Result{Stack: eventsdk.DetectedStack{Runtime: "ruby", Framework: "rails", Database: []string{}, Port: 3000}, Confidence: 0.90}
	applyDefaultCommands(&result.Stack)
	return result, nil
}

func collectDatabaseHints(path string, hints map[string]struct{}) error {
	if !fileExists(path) {
		return nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	lower := strings.ToLower(string(contents))
	for _, database := range []string{"postgres", "mysql", "mongodb", "redis"} {
		needle := database
		if database == "postgres" {
			if strings.Contains(lower, "postgres://") || strings.Contains(lower, "postgres:") || strings.Contains(lower, "postgresql") {
				hints[database] = struct{}{}
			}
			continue
		}
		if database == "mongodb" {
			if strings.Contains(lower, "mongodb://") || strings.Contains(lower, "mongo:") || strings.Contains(lower, "mongodb") {
				hints[database] = struct{}{}
			}
			continue
		}
		if strings.Contains(lower, needle+"://") || strings.Contains(lower, needle+":") || strings.Contains(lower, needle) {
			hints[database] = struct{}{}
		}
	}
	return nil
}

func hasTests(root string) bool {
	var found bool
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || found || entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".test.js") || strings.HasSuffix(name, ".test.ts") || strings.HasSuffix(name, ".spec.js") || strings.HasSuffix(name, ".spec.ts") || strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_spec.rb") || name == "pytest.ini" {
			found = true
		}
		return nil
	})
	return found
}

func applyDefaultCommands(stack *eventsdk.DetectedStack) {
	switch stack.Runtime {
	case "node":
		stack.BuildCommand = "npm run build"
		stack.TestCommand = "npm test"
		if stack.Port == 0 {
			stack.Port = 3000
		}
	case "python":
		stack.BuildCommand = "python -m compileall ."
		stack.TestCommand = "pytest"
		if stack.Port == 0 {
			stack.Port = 8000
		}
	case "go":
		stack.BuildCommand = "go build ./..."
		stack.TestCommand = "go test ./..."
		if stack.Port == 0 {
			stack.Port = 8080
		}
	case "java":
		stack.BuildCommand = "mvn package"
		stack.TestCommand = "mvn test"
		if stack.Port == 0 {
			stack.Port = 8080
		}
	case "ruby":
		stack.BuildCommand = "bundle exec rake"
		stack.TestCommand = "bundle exec rspec"
		if stack.Port == 0 {
			stack.Port = 3000
		}
	}
}

func mergedDependencies(primary, secondary map[string]string) map[string]string {
	merged := make(map[string]string, len(primary)+len(secondary))
	for key, value := range secondary {
		merged[key] = value
	}
	for key, value := range primary {
		merged[key] = value
	}
	return merged
}

func hasDependency(dependencies map[string]string, dependency string) bool {
	_, ok := dependencies[dependency]
	return ok
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
