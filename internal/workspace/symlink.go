package workspace

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const (
	LinkTypeSymlink  = "symlink"
	LinkTypeJunction = "junction"
)

var (
	ErrNotLink     = errors.New("path is not a link")
	osSymlink      = os.Symlink
	createJunction = defaultCreateJunction
)

func CreateLink(target, link string) (string, error) {
	if err := osSymlink(target, link); err == nil {
		return LinkTypeSymlink, nil
	} else if runtime.GOOS == "windows" && isWindowsPermissionError(err) {
		if err := createJunction(target, link); err != nil {
			return "", err
		}
		return LinkTypeJunction, nil
	} else {
		return "", err
	}
}

func DetectLinkType(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return LinkTypeSymlink, nil
	}

	if runtime.GOOS == "windows" {
		if _, err := os.Readlink(path); err == nil {
			return LinkTypeJunction, nil
		}
	}

	return "", ErrNotLink
}

func RemoveLink(path string) error {
	if _, err := DetectLinkType(path); err != nil {
		return err
	}

	return os.Remove(path)
}

func defaultCreateJunction(target, link string) error {
	if runtime.GOOS != "windows" {
		return errors.New("junctions are only supported on windows")
	}

	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("create junction: %w: %s", err, message)
		}
		return fmt.Errorf("create junction: %w", err)
	}

	return nil
}

func isWindowsPermissionError(err error) bool {
	if runtime.GOOS != "windows" || err == nil {
		return false
	}

	if errors.Is(err, os.ErrPermission) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "privilege not held") ||
		strings.Contains(message, "a required privilege is not held by the client") ||
		strings.Contains(message, "access is denied")
}

func permissionDeniedError(message string) error {
	return &os.LinkError{
		Op:  "symlink",
		Old: "target",
		New: "link",
		Err: errors.New(message),
	}
}

