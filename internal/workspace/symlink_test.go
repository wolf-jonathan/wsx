package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCreateLinkCreatesDirectoryLink(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	linkPath := filepath.Join(t.TempDir(), "link")
	method, err := CreateLink(target, linkPath)
	if err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}

	if runtime.GOOS == "windows" {
		if method != LinkTypeSymlink && method != LinkTypeJunction {
			t.Fatalf("CreateLink() method = %q, want symlink or junction", method)
		}

		if method == LinkTypeSymlink && info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("Lstat(%q) mode = %v, want symlink bit for symlink method", linkPath, info.Mode())
		}
	} else {
		if method != LinkTypeSymlink {
			t.Fatalf("CreateLink() method = %q, want symlink", method)
		}

		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("Lstat(%q) mode = %v, want symlink bit", linkPath, info.Mode())
		}
	}
}

func TestCreateLinkFallsBackToJunctionOnWindowsPermissionError(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only fallback behavior")
	}

	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	linkPath := filepath.Join(t.TempDir(), "link")

	originalSymlink := osSymlink
	originalJunction := createJunction
	t.Cleanup(func() {
		osSymlink = originalSymlink
		createJunction = originalJunction
	})

	osSymlink = func(string, string) error {
		return permissionDeniedError("symlink privilege not held")
	}

	junctionCalled := false
	createJunction = func(gotTarget, gotLink string) error {
		junctionCalled = true
		if gotTarget != target {
			t.Fatalf("createJunction() target = %q, want %q", gotTarget, target)
		}
		if gotLink != linkPath {
			t.Fatalf("createJunction() link = %q, want %q", gotLink, linkPath)
		}
		return nil
	}

	method, err := CreateLink(target, linkPath)
	if err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	if !junctionCalled {
		t.Fatal("CreateLink() did not call junction fallback")
	}

	if method != LinkTypeJunction {
		t.Fatalf("CreateLink() method = %q, want junction", method)
	}
}

func TestRemoveLinkRemovesLinkButLeavesTarget(t *testing.T) {
	t.Parallel()

	targetRoot := t.TempDir()
	target := filepath.Join(targetRoot, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	linkRoot := t.TempDir()
	linkPath := filepath.Join(linkRoot, "repo-link")
	if _, err := CreateLink(target, linkPath); err != nil {
		t.Fatalf("CreateLink() error = %v", err)
	}

	linkType, err := DetectLinkType(linkPath)
	if err != nil {
		t.Fatalf("DetectLinkType() error = %v", err)
	}

	if runtime.GOOS == "windows" {
		if linkType != LinkTypeSymlink && linkType != LinkTypeJunction {
			t.Fatalf("DetectLinkType() = %q, want symlink or junction", linkType)
		}
	} else if linkType != LinkTypeSymlink {
		t.Fatalf("DetectLinkType() = %q, want symlink", linkType)
	}

	if err := RemoveLink(linkPath); err != nil {
		t.Fatalf("RemoveLink() error = %v", err)
	}

	if _, err := os.Lstat(linkPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat() error = %v, want os.ErrNotExist", err)
	}

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("Stat(target) error = %v, want target preserved", err)
	}
}

func TestRemoveLinkRejectsRegularDirectories(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "plain-dir")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	err := RemoveLink(path)
	if !errors.Is(err, ErrNotLink) {
		t.Fatalf("RemoveLink() error = %v, want ErrNotLink", err)
	}
}

