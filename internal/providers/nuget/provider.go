package nuget

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/managedcache"
)

const ProviderID = "nuget.http-cache"

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`)

func NewProvider(runner common.CommandRunner) core.Provider {
	return managedcache.NewProvider(config(), runner)
}
func NewSystemProvider() core.Provider { return NewProvider(common.SystemRunner{}) }

func config() managedcache.Config {
	return managedcache.Config{
		ProviderID: ProviderID, Executable: "dotnet", ItemID: "nuget-http-cache", ItemName: "NuGet HTTP cache", PlanID: "nuget-http-cache-clear-plan", ActionID: "nuget-http-cache-clear",
		VersionArguments: []string{"--version"}, ParseVersion: parseVersion,
		SupportedMessage:   "The .NET SDK supports NuGet HTTP-cache inspection and reviewed cleanup.",
		UnsupportedMessage: "The detected .NET SDK predates the supported NuGet locals command; no cleanup plan will be created.",
		Risk:               core.RiskSafe, RecoveryCost: core.RecoveryDownload, EstimatedKind: core.MeasurementEstimatedLogical,
		Consequence: "NuGet will clear only its HTTP-request cache. Package metadata may be downloaded again; global packages, temporary data, and plugin caches are outside this action.",
		ResolveRoot: func(ctx context.Context, runner common.CommandRunner, executable string) (string, error) {
			output, err := runner.Run(ctx, executable, "nuget", "locals", "http-cache", "--list", "--force-english-output")
			if err != nil {
				return "", fmt.Errorf("resolve NuGet HTTP cache: %w", err)
			}
			return parseListedRoot(output)
		},
		Clean: func(ctx context.Context, runner common.CommandRunner, executable string) error {
			if _, err := runner.Run(ctx, executable, "nuget", "locals", "http-cache", "--clear", "--force-english-output"); err != nil {
				return fmt.Errorf("clear NuGet HTTP cache: %w", err)
			}
			return nil
		},
	}
}

func parseVersion(output string) (string, bool, error) {
	matches := versionPattern.FindStringSubmatch(output)
	if len(matches) != 4 {
		return "", false, errors.New("unsupported .NET SDK version format")
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", false, err
	}
	return matches[1] + "." + matches[2] + "." + matches[3], major >= 6, nil
}

func parseListedRoot(output string) (string, error) {
	line := strings.TrimSpace(output)
	if index := strings.Index(line, ":"); index >= 0 {
		line = strings.TrimSpace(line[index+1:])
	}
	return common.ValidateStorageRoot(line, "NuGet HTTP cache")
}
