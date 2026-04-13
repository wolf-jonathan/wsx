package ai

import (
	"strings"
	"testing"
)

func TestRenderWorkspacePromptIncludesRepoSummariesAndTree(t *testing.T) {
	output := RenderWorkspacePrompt(WorkspacePrompt{
		WorkspaceName: "payments-debug",
		Repos: []PromptRepo{
			{
				Name: "auth-service",
				Root: "C:/repos/auth-service",
				Detection: RepoDetection{
					Language: "Go",
				},
			},
			{
				Name: "frontend",
				Root: "C:/repos/frontend",
				Detection: RepoDetection{
					Language:  "Node.js",
					Framework: "Next.js",
				},
			},
		},
		Tree: "payments-debug/\n├── auth-service/\n└── frontend/\n",
	})

	for _, snippet := range []string{
		`You are working in a multi-repo workspace called "payments-debug".`,
		"- auth-service (C:/repos/auth-service) - Go",
		"- frontend (C:/repos/frontend) - Node.js / Next.js",
		"Directory structure:",
		"payments-debug/",
		"auth-service/",
		"frontend/",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("RenderWorkspacePrompt() = %q, want substring %q", output, snippet)
		}
	}
}

func TestRenderWorkspacePromptFallsBackToUnknown(t *testing.T) {
	output := RenderWorkspacePrompt(WorkspacePrompt{
		WorkspaceName: "scratchpad",
		Repos: []PromptRepo{
			{
				Name:      "misc",
				Root:      "C:/repos/misc",
				Detection: RepoDetection{},
			},
		},
		Tree: "scratchpad/\n└── misc/\n",
	})

	if !strings.Contains(output, "- misc (C:/repos/misc) - Unknown") {
		t.Fatalf("RenderWorkspacePrompt() = %q, want Unknown fallback", output)
	}
}
