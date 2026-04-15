package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	favoritesStoreDirName  = "wsx"
	favoritesStoreFileName = "favorites.json"
	favoritesStoreVersion  = "1"
)

var userConfigDir = os.UserConfigDir

type Favorite struct {
	Name  string    `json:"name"`
	Path  string    `json:"path"`
	Added time.Time `json:"added"`
}

type FavoriteStore struct {
	Version   string     `json:"version"`
	Favorites []Favorite `json:"favorites"`
}

func LoadFavoriteStore() (FavoriteStore, error) {
	path, err := favoriteStorePath()
	if err != nil {
		return FavoriteStore{}, err
	}

	return loadFavoriteStore(path)
}

func SaveFavoriteStore(store FavoriteStore) error {
	path, err := favoriteStorePath()
	if err != nil {
		return err
	}

	return saveFavoriteStore(path, store)
}

func loadFavoriteStore(path string) (FavoriteStore, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FavoriteStore{}, err
	}

	var store FavoriteStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return FavoriteStore{}, err
	}

	if store.Version == "" {
		store.Version = favoritesStoreVersion
	}
	if store.Favorites == nil {
		store.Favorites = []Favorite{}
	}

	return store, nil
}

func saveFavoriteStore(path string, store FavoriteStore) error {
	if store.Version == "" {
		store.Version = favoritesStoreVersion
	}
	if store.Favorites == nil {
		store.Favorites = []Favorite{}
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func favoriteStorePath() (string, error) {
	configDir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(configDir) == "" {
		return "", errors.New("user config directory is empty")
	}

	return filepath.Join(configDir, favoritesStoreDirName, favoritesStoreFileName), nil
}

func (store FavoriteStore) Index(name string) int {
	for i, favorite := range store.Favorites {
		if sameFavoriteName(favorite.Name, name) {
			return i
		}
	}

	return -1
}

func (store FavoriteStore) Get(name string) (Favorite, bool) {
	index := store.Index(name)
	if index < 0 {
		return Favorite{}, false
	}

	return store.Favorites[index], true
}

func (store *FavoriteStore) Add(favorite Favorite) error {
	favorite.Name = strings.TrimSpace(favorite.Name)
	favorite.Path = strings.TrimSpace(favorite.Path)
	if favorite.Name == "" {
		return errors.New("favorite name cannot be empty")
	}
	if favorite.Path == "" {
		return errors.New("favorite path cannot be empty")
	}
	if store.Index(favorite.Name) >= 0 {
		return fmt.Errorf("favorite name conflict: %q already exists", favorite.Name)
	}

	store.Favorites = append(store.Favorites, favorite)
	if store.Version == "" {
		store.Version = favoritesStoreVersion
	}

	return nil
}

func (store *FavoriteStore) Remove(name string) (Favorite, bool) {
	index := store.Index(name)
	if index < 0 {
		return Favorite{}, false
	}

	removed := store.Favorites[index]
	store.Favorites = append(store.Favorites[:index], store.Favorites[index+1:]...)
	return removed, true
}

func sameFavoriteName(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}

	return left == right
}
