package generator

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnsupportedStack = errors.New("unsupported stack")
	ErrInvalidRequest   = errors.New("invalid pipeline generation request")
)

// StackInput describes the detected application stack used for workflow generation.
type StackInput struct {
	Runtime   string `json:"runtime"`
	Framework string `json:"framework"`
}

// Request contains the pipeline generation request payload.
type Request struct {
	ProjectSlug string     `json:"project_slug"`
	Provider    string     `json:"provider"`
	Stack       StackInput `json:"stack"`
}

// GeneratedFile is a path/content pair for generated workflow files.
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Response contains generated workflow artifacts and metadata.
type Response struct {
	Template string          `json:"template"`
	Files    []GeneratedFile `json:"files"`
}

// Generate builds CI/CD workflow files for supported Phase 2 stacks.
func Generate(request Request) (Response, error) {
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		provider = "github-actions"
	}
	if provider != "github-actions" {
		return Response{}, fmt.Errorf("%w: provider %q is not supported yet", ErrInvalidRequest, request.Provider)
	}

	projectSlug := strings.TrimSpace(request.ProjectSlug)
	if projectSlug == "" {
		return Response{}, fmt.Errorf("%w: project_slug is required", ErrInvalidRequest)
	}

	runtime := strings.ToLower(strings.TrimSpace(request.Stack.Runtime))
	framework := strings.ToLower(strings.TrimSpace(request.Stack.Framework))
	if runtime == "" || framework == "" {
		return Response{}, fmt.Errorf("%w: stack.runtime and stack.framework are required", ErrInvalidRequest)
	}

	switch {
	case runtime == "node" && framework == "nextjs":
		return generateNextJS(projectSlug), nil
	case runtime == "python" && framework == "fastapi":
		return generateFastAPI(projectSlug), nil
	default:
		return Response{}, fmt.Errorf("%w: runtime=%q framework=%q", ErrUnsupportedStack, runtime, framework)
	}
}

func generateNextJS(projectSlug string) Response {
	return Response{
		Template: "github-actions-nextjs",
		Files: []GeneratedFile{{
			Path: ".github/workflows/generated-ci.yml",
			Content: `name: ` + projectSlug + ` CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: npm ci
      - run: npm run lint
      - run: npm run build
`,
		}},
	}
}

func generateFastAPI(projectSlug string) Response {
	return Response{
		Template: "github-actions-fastapi",
		Files: []GeneratedFile{{
			Path: ".github/workflows/generated-ci.yml",
			Content: `name: ` + projectSlug + ` CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: '3.12'
      - run: pip install -r requirements.txt
      - run: pytest
`,
		}},
	}
}