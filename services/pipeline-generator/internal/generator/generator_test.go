package generator

import "testing"

func TestGenerateNextJS(t *testing.T) {
	response, err := Generate(Request{
		ProjectSlug: "sample-next",
		Provider:    "github-actions",
		Stack: StackInput{Runtime: "node", Framework: "nextjs"},
	})
	if err != nil {
		t.Fatalf("generate nextjs: %v", err)
	}
	if response.Template != "github-actions-nextjs" {
		t.Fatalf("expected github-actions-nextjs template, got %q", response.Template)
	}
	if len(response.Files) != 1 {
		t.Fatalf("expected 1 generated file, got %d", len(response.Files))
	}
}

func TestGenerateFastAPI(t *testing.T) {
	response, err := Generate(Request{
		ProjectSlug: "sample-fastapi",
		Provider:    "github-actions",
		Stack: StackInput{Runtime: "python", Framework: "fastapi"},
	})
	if err != nil {
		t.Fatalf("generate fastapi: %v", err)
	}
	if response.Template != "github-actions-fastapi" {
		t.Fatalf("expected github-actions-fastapi template, got %q", response.Template)
	}
	if len(response.Files) != 1 {
		t.Fatalf("expected 1 generated file, got %d", len(response.Files))
	}
}

func TestGenerateRejectsUnsupportedStack(t *testing.T) {
	_, err := Generate(Request{
		ProjectSlug: "sample-unsupported",
		Provider:    "github-actions",
		Stack: StackInput{Runtime: "go", Framework: "gin"},
	})
	if err == nil {
		t.Fatal("expected error for unsupported stack")
	}
}