package common

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ValidateWorkspaceRoot(path string) (string, error) {
	root, err := ValidateStorageRoot(path, "workspace")
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("read workspace root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("workspace root is not a regular directory")
	}
	return root, nil
}

func ValidateWorkspaceTarget(workspaceRoot, target, name string) (string, error) {
	root, err := ValidateWorkspaceRoot(workspaceRoot)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		return "", fmt.Errorf("%s target is not absolute", name)
	}
	target = filepath.Clean(target)
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("%s target escapes the approved workspace", name)
	}
	current := root
	for _, component := range splitPath(relative) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			break
		}
		if statErr != nil {
			return "", fmt.Errorf("read %s target: %w", name, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s target contains a symbolic link", name)
		}
	}
	return target, nil
}

func splitPath(path string) []string {
	return strings.Split(filepath.Clean(path), string(filepath.Separator))
}
