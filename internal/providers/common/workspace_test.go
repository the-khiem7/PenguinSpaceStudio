package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWorkspaceRootAndTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "project", "target")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	validatedRoot, err := ValidateWorkspaceRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if !SamePath(validatedRoot, root) {
		t.Fatalf("unexpected root: %q", validatedRoot)
	}
	validatedTarget, err := ValidateWorkspaceTarget(root, target, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if !SamePath(validatedTarget, target) {
		t.Fatalf("unexpected target: %q", validatedTarget)
	}
}

func TestValidateWorkspaceRootAndTargetRejectBroadAndEscapingPaths(t *testing.T) {
	root := t.TempDir()
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	if _, err := ValidateWorkspaceRoot(volumeRoot); err == nil {
		t.Fatal("filesystem root accepted as workspace")
	}
	if _, err := ValidateWorkspaceTarget(root, filepath.Join(filepath.Dir(root), "outside"), "fixture"); err == nil {
		t.Fatal("outside target accepted")
	}
}

func TestValidateWorkspaceTargetRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := ValidateWorkspaceTarget(root, filepath.Join(link, "target"), "fixture"); err == nil {
		t.Fatal("symlink target accepted")
	}
}
