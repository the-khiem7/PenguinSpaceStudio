package common

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CommandRunner interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, executable string, arguments ...string) (string, error)
}

type SystemRunner struct{}

func (SystemRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (SystemRunner) Run(ctx context.Context, executable string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, executable, arguments...)
	configureCommand(command)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, message)
	}
	return string(output), nil
}

func NewCommandContext(name string) (string, func(), error) {
	directory, err := os.MkdirTemp("", "penguinspace-"+name+"-")
	if err != nil {
		return "", nil, err
	}
	return directory, func() { _ = os.RemoveAll(directory) }, nil
}

func ValidateStorageRoot(output, providerName string) (string, error) {
	root := strings.TrimSpace(output)
	if root == "" || strings.ContainsAny(root, "\r\n") {
		return "", fmt.Errorf("%s returned an invalid storage path", providerName)
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("%s storage path is not absolute", providerName)
	}
	root = filepath.Clean(root)
	volumeRoot := filepath.VolumeName(root) + string(filepath.Separator)
	if filepath.Clean(volumeRoot) == root {
		return "", fmt.Errorf("%s storage path resolves to a filesystem root", providerName)
	}
	if home, err := os.UserHomeDir(); err == nil && SamePath(root, home) {
		return "", fmt.Errorf("%s storage path resolves to the user home directory", providerName)
	}
	return root, nil
}

func MeasureDirectory(ctx context.Context, root string) (uint64, error) {
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return 0, errors.New("storage root is not a regular directory")
	}

	var total uint64
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path != root && entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() {
			return nil
		}
		size := uint64(fileInfo.Size())
		if math.MaxUint64-total < size {
			return errors.New("storage measurement overflow")
		}
		total += size
		return nil
	})
	return total, err
}

func SamePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
