package cypress

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

const ProviderID = "cypress.binary-cache"

var versionPattern = regexp.MustCompile(`(?:^|[^0-9])(\d+)\.(\d+)\.(\d+)(?:[^0-9]|$)`)

func NewProvider(runner common.CommandRunner) core.Provider {
	return managedcache.NewProvider(config(), runner)
}
func NewSystemProvider() core.Provider { return NewProvider(common.SystemRunner{}) }

func config() managedcache.Config {
	return managedcache.Config{
		ProviderID: ProviderID, Executable: "cypress", ItemID: "cypress-binary-cache", ItemName: "Cypress binary cache", PlanID: "cypress-cache-prune-plan", ActionID: "cypress-cache-prune",
		VersionArguments: []string{"--version"}, ParseVersion: parseVersion,
		SupportedMessage:   "Cypress 13.x through 15.x is supported for binary-cache inspection and pruning.",
		UnsupportedMessage: "Cypress was detected, but this version is outside the supported 13.x–15.x range; no cleanup plan will be created.",
		Risk:               core.RiskSafe, RecoveryCost: core.RecoveryDownload, EstimatedKind: core.MeasurementUnavailable,
		Consequence: "Cypress will remove cached binary versions other than the version currently in use. The displayed cache size is not a reclaim estimate; a retained or future binary may be downloaded again.",
		ResolveRoot: func(ctx context.Context, runner common.CommandRunner, executable string) (string, error) {
			output, err := runner.Run(ctx, executable, "cache", "path")
			if err != nil {
				return "", fmt.Errorf("resolve Cypress binary cache: %w", err)
			}
			return common.ValidateStorageRoot(output, "Cypress")
		},
		Clean: func(ctx context.Context, runner common.CommandRunner, executable string) error {
			if _, err := runner.Run(ctx, executable, "cache", "prune"); err != nil {
				return fmt.Errorf("prune Cypress binary cache: %w", err)
			}
			return nil
		},
	}
}

func parseVersion(output string) (string, bool, error) {
	matches := versionPattern.FindStringSubmatch(output)
	if len(matches) != 4 {
		return "", false, errors.New("unsupported Cypress version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", false, err
	}
	return matches[1] + "." + matches[2] + "." + matches[3], major >= 13 && major <= 15, nil
}
