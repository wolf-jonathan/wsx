package workspace

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	ErrUnresolvedVariable = errors.New("unresolved variable")
	varPattern            = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

type EnvVars map[string]string

func LoadEnv(root string) (EnvVars, error) {
	file, err := os.Open(filepath.Join(root, EnvFileName))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	env := EnvVars{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid env line %q", line)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("invalid env line %q", line)
		}

		env[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return env, nil
}

func ResolvePath(path string, env EnvVars) (string, error) {
	if path == "" {
		return "", nil
	}

	var resolveErr error
	resolved := varPattern.ReplaceAllStringFunc(path, func(match string) string {
		if resolveErr != nil {
			return match
		}

		groups := varPattern.FindStringSubmatch(match)
		if len(groups) != 2 {
			return match
		}

		name := groups[1]
		if env != nil {
			if value, ok := env[name]; ok {
				return value
			}
		}

		if value, ok := os.LookupEnv(name); ok {
			return value
		}

		resolveErr = fmt.Errorf("%w: %s", ErrUnresolvedVariable, name)
		return match
	})

	if resolveErr != nil {
		return "", resolveErr
	}

	return filepath.Clean(filepath.FromSlash(resolved)), nil
}

func SaveEnv(root string, env EnvVars) error {
	keys := make([]string, 0, len(env))
	for key := range env {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(env[key])
		builder.WriteByte('\n')
	}

	return os.WriteFile(filepath.Join(root, EnvFileName), []byte(builder.String()), 0o644)
}
