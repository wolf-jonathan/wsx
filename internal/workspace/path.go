package workspace

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrLegacyPlaceholderPath = errors.New("legacy placeholder-based workspace config is no longer supported")
	ErrRelativeRefPath       = errors.New("workspace ref path must be absolute")
	placeholderPattern       = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*\}`)
)

func ResolveInputPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path cannot be empty")
	}
	if placeholderPattern.MatchString(path) {
		return "", errors.New("placeholder paths are no longer supported; use an absolute path or --favorite")
	}
	if !filepath.IsAbs(path) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = absolute
	}

	return filepath.Clean(path), nil
}

func ResolveStoredPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("ref path cannot be empty")
	}
	if placeholderPattern.MatchString(path) {
		return "", fmt.Errorf("%w; re-add refs with absolute paths", ErrLegacyPlaceholderPath)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: %s", ErrRelativeRefPath, path)
	}

	return filepath.Clean(path), nil
}
