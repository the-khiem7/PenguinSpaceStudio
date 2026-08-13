package playwright

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/projectcleanup"
)

const ProviderID = "playwright.hermetic-browsers"

var versionPattern = regexp.MustCompile(`(?:Version\s+)?(\d+)\.(\d+)\.(\d+)`)

func NewProvider(runner common.CommandRunner) *projectcleanup.Provider {
	return projectcleanup.NewProvider(config(), runner)
}
func NewSystemProvider() core.Provider { return NewProvider(common.SystemRunner{}) }
func config() projectcleanup.Config {
	return projectcleanup.Config{
		ProviderID: ProviderID, ItemID: "playwright-hermetic-browsers", ItemName: "Playwright hermetic browsers", PlanID: "playwright-hermetic-uninstall-plan", ActionID: "playwright-hermetic-uninstall",
		ExecutableName:     "Playwright local CLI",
		FindExecutable:     findLocalCLI,
		VersionArguments:   func(string) []string { return []string{"--version"} },
		CleanupArguments:   func(string) []string { return []string{"uninstall"} },
		CleanupEnvironment: func(string) []string { return []string{"PLAYWRIGHT_BROWSERS_PATH=0"} },
		ParseVersion:       parseVersion,
		ValidateWorkspace:  validateWorkspace,
		ResolveTarget: func(root string) string {
			return filepath.Join(root, "node_modules", "playwright-core", ".local-browsers")
		},
		SupportedMessage:   "Playwright 1.40 or later hermetic local browsers are supported for approved workspace cleanup.",
		UnsupportedMessage: "Playwright was detected, but this version is outside the supported 1.40+ range; no cleanup plan will be created.",
		Consequence:        "Playwright will uninstall only this workspace's hermetic browsers. They will be downloaded again when needed; shared OS browser caches and --all removal are outside this action.",
	}
}
func findLocalCLI(root string, _ common.CommandRunner) (string, error) {
	name := "playwright"
	if runtime.GOOS == "windows" {
		name += ".cmd"
	}
	path := filepath.Join(root, "node_modules", ".bin", name)
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("Playwright CLI is not a regular workspace file")
	}
	return path, nil
}
func validateWorkspace(root string) error {
	return projectcleanup.RequireRegularWorkspaceFile(root, "package.json", "Playwright")
}
func parseVersion(output string) (string, bool, error) {
	matches := versionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(matches) != 4 {
		return "", false, errors.New("unsupported Playwright version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", false, err
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return "", false, err
	}
	return matches[1] + "." + matches[2] + "." + matches[3], major > 1 || major == 1 && minor >= 40, nil
}
