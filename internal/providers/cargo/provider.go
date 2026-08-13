package cargo

import (
	"errors"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/projectcleanup"
)

const ProviderID = "cargo.workspace-target"

var versionPattern = regexp.MustCompile(`^cargo\s+(\d+)\.(\d+)\.(\d+)`)

func NewProvider(runner common.CommandRunner) *projectcleanup.Provider {
	return projectcleanup.NewProvider(config(), runner)
}
func NewSystemProvider() core.Provider { return NewProvider(common.SystemRunner{}) }

func config() projectcleanup.Config {
	return projectcleanup.Config{
		ProviderID: ProviderID, ItemID: "cargo-workspace-target", ItemName: "Cargo workspace target", PlanID: "cargo-workspace-clean-plan", ActionID: "cargo-workspace-clean",
		ExecutableName:   "cargo",
		FindExecutable:   func(_ string, runner common.CommandRunner) (string, error) { return runner.LookPath("cargo") },
		VersionArguments: func(string) []string { return []string{"--version"} },
		CleanupArguments: func(root string) []string {
			return []string{"clean", "--manifest-path", root + string(filepath.Separator) + "Cargo.toml"}
		},
		ParseVersion: parseVersion,
		ValidateWorkspace: func(root string) error {
			return projectcleanup.RequireRegularWorkspaceFile(root, "Cargo.toml", "Cargo")
		},
		ResolveTarget:      func(root string) string { return root + string(filepath.Separator) + "target" },
		SupportedMessage:   "Cargo 1.70 or later is supported for approved workspace target cleanup.",
		UnsupportedMessage: "Cargo was detected, but this version is outside the supported 1.70+ range; no cleanup plan will be created.",
		Consequence:        "Cargo will remove generated artifacts in this approved workspace target directory. The project will rebuild them as needed; Cargo home caches are outside this action.",
	}
}
func parseVersion(output string) (string, bool, error) {
	matches := versionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(matches) != 4 {
		return "", false, errors.New("unsupported Cargo version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", false, err
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return "", false, err
	}
	return matches[1] + "." + matches[2] + "." + matches[3], major > 1 || major == 1 && minor >= 70, nil
}
