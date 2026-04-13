package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/wolf-jonathan/workspace-x/internal/workspace"
)

func validateWorkspaceLinkTarget(root, name, resolvedTarget string) (string, error) {
	linkPath := filepath.Join(root, name)

	linkType, err := workspace.DetectLinkType(linkPath)
	if err != nil {
		return "", fmt.Errorf("workspace entry is missing or not a link: %s", linkPath)
	}

	actualTarget, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		return "", fmt.Errorf("workspace link could not be resolved: %s", linkPath)
	}

	if !samePath(actualTarget, resolvedTarget) {
		return "", fmt.Errorf("workspace link points to %s instead of %s", actualTarget, resolvedTarget)
	}

	return linkType, nil
}
