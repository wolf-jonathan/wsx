package workspace

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLoadFavoriteStoreReturnsNotExistWhenMissing(t *testing.T) {
	withFavoriteConfigDir(t, t.TempDir())

	_, err := LoadFavoriteStore()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadFavoriteStore() error = %v, want os.ErrNotExist", err)
	}
}

func TestSaveFavoriteStoreWritesExpectedSchema(t *testing.T) {
	configDir := t.TempDir()
	withFavoriteConfigDir(t, configDir)

	created := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	store := FavoriteStore{
		Favorites: []Favorite{
			{
				Name:  "WORK_REPOS",
				Path:  `C:\src\repos`,
				Added: created,
			},
		},
	}

	if err := SaveFavoriteStore(store); err != nil {
		t.Fatalf("SaveFavoriteStore() error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(configDir, favoritesStoreDirName, favoritesStoreFileName))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var decoded FavoriteStore
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Version != favoritesStoreVersion {
		t.Fatalf("Version = %q, want %q", decoded.Version, favoritesStoreVersion)
	}

	if len(decoded.Favorites) != 1 {
		t.Fatalf("Favorites length = %d, want 1", len(decoded.Favorites))
	}

	favorite := decoded.Favorites[0]
	if favorite.Name != "WORK_REPOS" {
		t.Fatalf("Favorite.Name = %q, want WORK_REPOS", favorite.Name)
	}
	if favorite.Path != `C:\src\repos` {
		t.Fatalf("Favorite.Path = %q, want C:\\src\\repos", favorite.Path)
	}
	if !favorite.Added.Equal(created) {
		t.Fatalf("Favorite.Added = %s, want %s", favorite.Added, created)
	}
}

func TestFavoriteStoreAddGetRemove(t *testing.T) {
	store := FavoriteStore{}
	added := time.Date(2026, 4, 15, 12, 5, 0, 0, time.UTC)

	if err := store.Add(Favorite{
		Name:  "WORK_REPOS",
		Path:  `C:\src\repos`,
		Added: added,
	}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got, ok := store.Get("WORK_REPOS")
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if got.Name != "WORK_REPOS" {
		t.Fatalf("Get() name = %q, want WORK_REPOS", got.Name)
	}

	removed, ok := store.Remove("WORK_REPOS")
	if !ok {
		t.Fatal("Remove() ok = false, want true")
	}
	if removed.Path != `C:\src\repos` {
		t.Fatalf("Remove() path = %q, want C:\\src\\repos", removed.Path)
	}
	if len(store.Favorites) != 0 {
		t.Fatalf("Favorites length = %d, want 0", len(store.Favorites))
	}
}

func TestFavoriteStoreNameMatchingHonorsWindowsCaseInsensitivity(t *testing.T) {
	store := FavoriteStore{}
	if err := store.Add(Favorite{
		Name:  "WORK_REPOS",
		Path:  `C:\src\repos`,
		Added: time.Date(2026, 4, 15, 12, 10, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if runtime.GOOS == "windows" {
		if _, ok := store.Get("work_repos"); !ok {
			t.Fatal("Get() ok = false, want true on Windows")
		}

		if err := store.Add(Favorite{
			Name:  "work_repos",
			Path:  `C:\src\other`,
			Added: time.Date(2026, 4, 15, 12, 15, 0, 0, time.UTC),
		}); err == nil {
			t.Fatal("Add() error = nil, want case-collision error on Windows")
		}
		return
	}

	if _, ok := store.Get("work_repos"); ok {
		t.Fatal("Get() ok = true, want false off Windows")
	}

	if err := store.Add(Favorite{
		Name:  "work_repos",
		Path:  `C:\src\other`,
		Added: time.Date(2026, 4, 15, 12, 15, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Add() error = %v, want success off Windows", err)
	}
}

func withFavoriteConfigDir(t *testing.T, dir string) {
	t.Helper()

	previous := userConfigDir
	userConfigDir = func() (string, error) {
		return dir, nil
	}

	t.Cleanup(func() {
		userConfigDir = previous
	})
}
