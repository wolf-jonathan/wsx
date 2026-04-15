package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/wolf-jonathan/workspace-x/cmd"
	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

func TestFavoriteAddListAndRemove(t *testing.T) {
	configDir := t.TempDir()
	setFavoriteConfigDirEnv(t, configDir)

	favoriteRoot := filepath.Join(t.TempDir(), "repos")
	if err := os.MkdirAll(favoriteRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	add := cmd.NewRootCommand()
	add.SetArgs([]string{"favorite", "add", favoriteRoot, "--name", "WORK_REPOS"})
	add.SetOut(new(bytes.Buffer))
	add.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(add); err != nil {
		t.Fatalf("ExecuteCommand(add) error = %v", err)
	}

	storePath := filepath.Join(configDir, "wsx", "favorites.json")
	raw, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", storePath, err)
	}

	var store workspace.FavoriteStore
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(store.Favorites) != 1 {
		t.Fatalf("Favorites length = %d, want 1", len(store.Favorites))
	}

	stdout := new(bytes.Buffer)
	list := cmd.NewRootCommand()
	list.SetArgs([]string{"favorite", "list", "--json"})
	list.SetOut(stdout)
	list.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(list); err != nil {
		t.Fatalf("ExecuteCommand(list) error = %v", err)
	}

	var favorites []workspace.Favorite
	if err := json.Unmarshal(stdout.Bytes(), &favorites); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}

	if len(favorites) != 1 {
		t.Fatalf("favorites length = %d, want 1", len(favorites))
	}

	favorite := favorites[0]
	if favorite.Name != "WORK_REPOS" {
		t.Fatalf("favorite.Name = %q, want WORK_REPOS", favorite.Name)
	}
	if favorite.Path != favoriteRoot {
		t.Fatalf("favorite.Path = %q, want %q", favorite.Path, favoriteRoot)
	}
	if favorite.Added.IsZero() {
		t.Fatal("favorite.Added = zero, want timestamp")
	}

	remove := cmd.NewRootCommand()
	remove.SetArgs([]string{"favorite", "remove", "WORK_REPOS"})
	remove.SetOut(new(bytes.Buffer))
	remove.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(remove); err != nil {
		t.Fatalf("ExecuteCommand(remove) error = %v", err)
	}

	stdout.Reset()
	list = cmd.NewRootCommand()
	list.SetArgs([]string{"favorite", "list", "--json"})
	list.SetOut(stdout)
	list.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(list); err != nil {
		t.Fatalf("ExecuteCommand(list after remove) error = %v", err)
	}

	if err := json.Unmarshal(stdout.Bytes(), &favorites); err != nil {
		t.Fatalf("json.Unmarshal(list after remove) error = %v", err)
	}
	if len(favorites) != 0 {
		t.Fatalf("favorites length after remove = %d, want 0", len(favorites))
	}
}

func setFavoriteConfigDirEnv(t *testing.T, dir string) {
	t.Helper()

	for _, key := range []string{
		"APPDATA",
		"LOCALAPPDATA",
		"XDG_CONFIG_HOME",
		"HOME",
		"USERPROFILE",
	} {
		t.Setenv(key, dir)
	}
}

func TestFavoriteAddRejectsWindowsNameCollision(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific name collision behavior")
	}

	configDir := t.TempDir()
	setFavoriteConfigDirEnv(t, configDir)

	target := filepath.Join(t.TempDir(), "repos")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	first := cmd.NewRootCommand()
	first.SetArgs([]string{"favorite", "add", target, "--name", "WORK_REPOS"})
	first.SetOut(new(bytes.Buffer))
	first.SetErr(new(bytes.Buffer))
	if err := cmd.ExecuteCommand(first); err != nil {
		t.Fatalf("ExecuteCommand(first add) error = %v", err)
	}

	second := cmd.NewRootCommand()
	second.SetArgs([]string{"favorite", "add", target, "--name", "work_repos"})
	second.SetOut(new(bytes.Buffer))
	second.SetErr(new(bytes.Buffer))

	err := cmd.ExecuteCommand(second)
	if err == nil {
		t.Fatal("ExecuteCommand(second add) error = nil, want case-collision error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "conflict") {
		t.Fatalf("error = %q, want conflict message", err.Error())
	}
}
