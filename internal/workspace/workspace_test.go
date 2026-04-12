package workspace_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jwolf/wsx/internal/workspace"
)

func TestSaveConfigWritesExpectedSchema(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	created := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	added := created.Add(5 * time.Minute)

	cfg := workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Created: created,
		Refs: []workspace.Ref{
			{
				Name:  "auth-service",
				Path:  "${WORK_REPOS}/auth-service",
				Added: added,
			},
		},
	}

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, workspace.ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got := decoded["name"]; got != "payments-debug" {
		t.Fatalf("name = %v, want payments-debug", got)
	}

	refs, ok := decoded["refs"].([]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("refs = %T %#v, want one ref", decoded["refs"], decoded["refs"])
	}

	ref, ok := refs[0].(map[string]any)
	if !ok {
		t.Fatalf("refs[0] = %T, want object", refs[0])
	}

	if got := ref["path"]; got != "${WORK_REPOS}/auth-service" {
		t.Fatalf("stored path = %v, want placeholder form preserved", got)
	}
}

func TestLoadConfigDiscoversWorkspaceByWalkingUp(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	cfg := workspace.Config{
		Version: "1",
		Name:    "payments-debug",
		Created: time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC),
	}

	if err := workspace.SaveConfig(root, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	loaded, err := workspace.LoadConfig(nested)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if loaded.Root != root {
		t.Fatalf("Root = %q, want %q", loaded.Root, root)
	}

	if loaded.Config.Name != "payments-debug" {
		t.Fatalf("Config.Name = %q, want payments-debug", loaded.Config.Name)
	}
}

func TestLoadConfigReturnsNotFoundWhenWorkspaceMissing(t *testing.T) {
	t.Parallel()

	_, err := workspace.LoadConfig(t.TempDir())
	if !errors.Is(err, workspace.ErrWorkspaceNotFound) {
		t.Fatalf("LoadConfig() error = %v, want ErrWorkspaceNotFound", err)
	}
}
