package generator

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnsupportedStack = errors.New("unsupported stack")
	ErrInvalidRequest   = errors.New("invalid infra generation request")
)

// StackInput describes the detected application stack used for template generation.
type StackInput struct {
	Runtime   string   `json:"runtime"`
	Framework string   `json:"framework"`
	Database  []string `json:"database"`
	Port      int      `json:"port"`
}

// Request is the payload accepted by the generate endpoint.
type Request struct {
	ProjectSlug string     `json:"project_slug"`
	Provider    string     `json:"provider"`
	Stack       StackInput `json:"stack"`
}

// GeneratedFile is a path/content pair for generated infrastructure artifacts.
type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Response contains generated files and metadata.
type Response struct {
	Template string          `json:"template"`
	Files    []GeneratedFile `json:"files"`
}

// Generate builds infrastructure files for supported Phase 2 templates.
func Generate(request Request) (Response, error) {
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		provider = "docker"
	}
	if provider != "docker" {
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

	port := request.Stack.Port
	if port == 0 {
		port = defaultPort(runtime, framework)
	}

	switch {
	case runtime == "node" && framework == "nextjs":
		return generateNextJS(projectSlug, port), nil
	case runtime == "python" && framework == "fastapi":
		return generateFastAPI(projectSlug, port), nil
	default:
		return Response{}, fmt.Errorf("%w: runtime=%q framework=%q", ErrUnsupportedStack, runtime, framework)
	}
}

func generateNextJS(projectSlug string, port int) Response {
	return Response{
		Template: "docker-nextjs",
		Files: []GeneratedFile{
			{
				Path: "infra/docker/Dockerfile",
				Content: `FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build
EXPOSE ` + fmt.Sprintf("%d", port) + `
CMD ["npm", "run", "start"]
`,
			},
			{
				Path: "infra/docker/docker-compose.generated.yml",
				Content: `services:
  ` + projectSlug + `:
    build: .
    ports:
      - "` + fmt.Sprintf("%d", port) + `:` + fmt.Sprintf("%d", port) + `"
`,
			},
		},
	}
}

func generateFastAPI(projectSlug string, port int) Response {
	return Response{
		Template: "docker-fastapi",
		Files: []GeneratedFile{
			{
				Path: "infra/docker/Dockerfile",
				Content: `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE ` + fmt.Sprintf("%d", port) + `
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "` + fmt.Sprintf("%d", port) + `"]
`,
			},
			{
				Path: "infra/docker/docker-compose.generated.yml",
				Content: `services:
  ` + projectSlug + `:
    build: .
    ports:
      - "` + fmt.Sprintf("%d", port) + `:` + fmt.Sprintf("%d", port) + `"
`,
			},
		},
	}
}

func defaultPort(runtime, framework string) int {
	switch {
	case runtime == "node" && framework == "nextjs":
		return 3000
	case runtime == "python" && framework == "fastapi":
		return 8000
	default:
		return 8080
	}
}
