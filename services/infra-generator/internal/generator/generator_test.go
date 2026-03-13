package generator

import "testing"

func TestGenerateNextJS(t *testing.T) {
	response, err := Generate(Request{
		ProjectSlug: "sample-next",
		Provider:    "docker",
		Stack: StackInput{
			Runtime:   "node",
			Framework: "nextjs",
		},
	})
	if err != nil {
		t.Fatalf("generate nextjs: %v", err)
	}
	if response.Template != "docker-nextjs" {
		t.Fatalf("expected docker-nextjs template, got %q", response.Template)
	}
	if len(response.Files) != 2 {
		t.Fatalf("expected 2 generated files, got %d", len(response.Files))
	}
}

func TestGenerateFastAPI(t *testing.T) {
	response, err := Generate(Request{
		ProjectSlug: "sample-fastapi",
		Provider:    "docker",
		Stack: StackInput{
			Runtime:   "python",
			Framework: "fastapi",
		},
	})
	if err != nil {
		t.Fatalf("generate fastapi: %v", err)
	}
	if response.Template != "docker-fastapi" {
		t.Fatalf("expected docker-fastapi template, got %q", response.Template)
	}
	if len(response.Files) != 2 {
		t.Fatalf("expected 2 generated files, got %d", len(response.Files))
	}
}

func TestGenerateRejectsUnsupportedStack(t *testing.T) {
	_, err := Generate(Request{
		ProjectSlug: "sample-unsupported",
		Provider:    "docker",
		Stack: StackInput{
			Runtime:   "go",
			Framework: "gin",
		},
	})
	if err == nil {
		t.Fatal("expected error for unsupported stack")
	}
}
