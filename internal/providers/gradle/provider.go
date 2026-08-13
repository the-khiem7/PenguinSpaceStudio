package gradle

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/projectcleanup"
)

const ProviderID = "gradle.workspace-build"

var versionPattern = regexp.MustCompile(`(?m)^Gradle\s+(\d+)\.(\d+)(?:\.(\d+))?`)

func NewProvider(runner common.CommandRunner) *projectcleanup.Provider {
	return projectcleanup.NewProvider(config(), runner)
}
func NewSystemProvider() core.Provider { return NewProvider(common.SystemRunner{}) }

func config() projectcleanup.Config {
	return projectcleanup.Config{
		ProviderID: ProviderID, ItemID: "gradle-workspace-build", ItemName: "Gradle workspace build output", PlanID: "gradle-workspace-clean-plan", ActionID: "gradle-workspace-clean",
		FindExecutable:     findWrapper,
		VersionArguments:   func(string) []string { return []string{"--version"} },
		CleanupArguments:   func(root string) []string { return []string{"--no-daemon", "-p", root, ":clean"} },
		ParseVersion:       parseVersion,
		ValidateWorkspace:  validateWorkspace,
		ResolveTarget:      func(root string) string { return filepath.Join(root, "build") },
		SupportedMessage:   "Gradle 8.x or 9.x wrapper is supported for approved root-project build cleanup.",
		UnsupportedMessage: "Gradle was detected, but this version is outside the supported 8.x–9.x range; no cleanup plan will be created.",
		Consequence:        "Gradle will run the root project's clean task for this approved workspace. Root build output will be rebuilt; Gradle User Home and subproject outputs are outside this action.",
	}
}
func findWrapper(root string, _ common.CommandRunner) (string, error) {
	name := "gradlew"
	if strings.EqualFold(filepath.Ext(os.Args[0]), ".exe") {
		name = "gradlew.bat"
	}
	path := filepath.Join(root, name)
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("Gradle wrapper is not a regular workspace file")
	}
	return path, nil
}
func validateWorkspace(root string) error {
	for _, name := range []string{"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"} {
		if err := projectcleanup.RequireRegularWorkspaceFile(root, name, "Gradle"); err == nil {
			return nil
		}
	}
	return errors.New("approved workspace does not contain a regular Gradle build or settings file")
}
func parseVersion(output string) (string, bool, error) {
	matches := versionPattern.FindStringSubmatch(output)
	if len(matches) != 4 {
		return "", false, errors.New("unsupported Gradle version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", false, err
	}
	if _, err := strconv.Atoi(matches[2]); err != nil {
		return "", false, err
	}
	patch := matches[3]
	if patch == "" {
		patch = "0"
	}
	return matches[1] + "." + matches[2] + "." + patch, major == 8 || major == 9, nil
}
