package maven

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

const ProviderID = "maven.workspace-target"

var versionPattern = regexp.MustCompile(`(?m)^Apache Maven\s+(\d+)\.(\d+)\.(\d+)`)

func NewProvider(runner common.CommandRunner) *projectcleanup.Provider {
	return projectcleanup.NewProvider(config(), runner)
}
func NewSystemProvider() core.Provider { return NewProvider(common.SystemRunner{}) }
func config() projectcleanup.Config {
	return projectcleanup.Config{
		ProviderID: ProviderID, ItemID: "maven-workspace-target", ItemName: "Maven workspace target", PlanID: "maven-workspace-clean-plan", ActionID: "maven-workspace-clean",
		FindExecutable:     func(_ string, runner common.CommandRunner) (string, error) { return runner.LookPath("mvn") },
		VersionArguments:   func(string) []string { return []string{"--version"} },
		CleanupArguments:   func(root string) []string { return []string{"-f", filepath.Join(root, "pom.xml"), "clean"} },
		ParseVersion:       parseVersion,
		ValidateWorkspace:  func(root string) error { return projectcleanup.RequireRegularWorkspaceFile(root, "pom.xml", "Maven") },
		ResolveTarget:      func(root string) string { return filepath.Join(root, "target") },
		SupportedMessage:   "Apache Maven 3.x or 4.x is supported for approved workspace target cleanup.",
		UnsupportedMessage: "Maven was detected, but this version is outside the supported 3.x–4.x range; no cleanup plan will be created.",
		Consequence:        "Maven will run clean for this approved workspace target directory. Generated output will be rebuilt; the shared Maven local repository is outside this action.",
	}
}
func parseVersion(output string) (string, bool, error) {
	matches := versionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(matches) != 4 {
		return "", false, errors.New("unsupported Maven version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", false, err
	}
	return matches[1] + "." + matches[2] + "." + matches[3], major == 3 || major == 4, nil
}
