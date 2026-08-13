package yarn

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/managedcache"
)

const ProviderID = "yarn.classic-global-cache"

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$`)

func NewProvider(runner common.CommandRunner) core.Provider {
	return managedcache.NewProvider(config(), runner)
}
func NewSystemProvider() core.Provider { return NewProvider(common.SystemRunner{}) }

func config() managedcache.Config {
	return managedcache.Config{
		ProviderID: ProviderID, Executable: "yarn", ItemID: "yarn-classic-global-cache", ItemName: "Yarn Classic global cache", PlanID: "yarn-classic-cache-clean-plan", ActionID: "yarn-classic-cache-clean",
		VersionArguments: []string{"--version"}, ParseVersion: parseVersion,
		SupportedMessage:   "Yarn Classic 1.x is supported for global-cache inspection and reviewed cleanup.",
		UnsupportedMessage: "Yarn was detected, but only Classic 1.x is supported. Modern Yarn can use project-local or shared caches and needs explicit project scope.",
		Risk:               core.RiskReview, RecoveryCost: core.RecoveryDownload, EstimatedKind: core.MeasurementEstimatedLogical,
		Consequence: "Yarn Classic will clear its global cache. Packages may be downloaded again; Modern Yarn project-local caches are deliberately outside this action.",
		ResolveRoot: func(ctx context.Context, runner common.CommandRunner, executable string) (string, error) {
			output, err := runner.Run(ctx, executable, "cache", "dir")
			if err != nil {
				return "", fmt.Errorf("resolve Yarn Classic global cache: %w", err)
			}
			return common.ValidateStorageRoot(output, "Yarn Classic")
		},
		Clean: func(ctx context.Context, runner common.CommandRunner, executable string) error {
			if _, err := runner.Run(ctx, executable, "cache", "clean"); err != nil {
				return fmt.Errorf("clean Yarn Classic global cache: %w", err)
			}
			return nil
		},
	}
}

func parseVersion(output string) (string, bool, error) {
	matches := versionPattern.FindStringSubmatch(output)
	if len(matches) != 4 {
		return "", false, errors.New("unsupported Yarn version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", false, err
	}
	return matches[1] + "." + matches[2] + "." + matches[3], major == 1, nil
}
