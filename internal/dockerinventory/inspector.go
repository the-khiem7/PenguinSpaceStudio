package dockerinventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

var sizePattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)([kMGTPE]?B)$`)

type Inspector struct {
	runner common.CommandRunner
}

func NewSystemInspector() *Inspector {
	return &Inspector{runner: common.SystemRunner{}}
}

func NewInspector(runner common.CommandRunner) *Inspector {
	return &Inspector{runner: runner}
}

func (i *Inspector) Inspect(ctx context.Context) core.DockerAwareness {
	report := core.DockerAwareness{InspectedAt: time.Now().UTC()}
	executable, err := i.runner.LookPath("docker")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			report.Daemon.Message = "Docker CLI was not found on PATH. No daemon resources were inspected."
			return report
		}
		report.Daemon.Message = fmt.Sprintf("Docker CLI lookup failed: %v", err)
		return report
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		report.Daemon.Message = fmt.Sprintf("Docker CLI path could not be resolved: %v", err)
		return report
	}
	report.Daemon.CLIAvailable = true
	report.Daemon.ExecutablePath = executable

	versionOutput, err := i.runner.Run(ctx, executable, "version", "--format", "{{json .Server}}")
	if err != nil {
		report.Daemon.Message = fmt.Sprintf("Docker CLI is available, but the daemon could not be reached: %v", err)
		return report
	}
	var server struct {
		Version string `json:"Version"`
		OS      string `json:"Os"`
		Arch    string `json:"Arch"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(versionOutput)), &server); err != nil || server.Version == "" {
		report.Daemon.Message = "Docker daemon responded with an unrecognised version payload."
		return report
	}
	report.Daemon.Available = true
	report.Daemon.Version = server.Version
	report.Daemon.OperatingSystem = server.OS
	report.Daemon.Architecture = server.Arch
	report.Daemon.Message = "Docker daemon is available. Resource inspection is read-only."

	diskUsage := i.inspectDiskUsage(ctx, executable, &report)
	report.Resources = []core.DockerResourceSummary{
		i.inspectImages(ctx, executable, diskUsage["Images"], &report),
		i.inspectStoppedContainers(ctx, executable, &report),
		i.inspectBuildCache(ctx, executable, diskUsage["Build Cache"], &report),
		i.inspectNetworks(ctx, executable, &report),
		i.inspectVolumes(ctx, executable, diskUsage["Local Volumes"], &report),
	}
	return report
}

type diskUsageRow struct {
	Size        core.Measurement
	Reclaimable core.Measurement
}

func (i *Inspector) inspectDiskUsage(ctx context.Context, executable string, report *core.DockerAwareness) map[string]diskUsageRow {
	rows := make(map[string]diskUsageRow)
	output, err := i.runner.Run(ctx, executable, "system", "df", "--format", "json")
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Docker disk-usage summary was unavailable: %v", err))
		return rows
	}
	for _, line := range nonEmptyLines(output) {
		var value struct {
			Type        string `json:"Type"`
			Size        string `json:"Size"`
			Reclaimable string `json:"Reclaimable"`
		}
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			report.Warnings = append(report.Warnings, "Docker returned a malformed disk-usage row.")
			continue
		}
		rows[value.Type] = diskUsageRow{
			Size:        parseSize(value.Size),
			Reclaimable: parseSize(firstField(value.Reclaimable)),
		}
	}
	return rows
}

func (i *Inspector) inspectImages(ctx context.Context, executable string, usage diskUsageRow, report *core.DockerAwareness) core.DockerResourceSummary {
	result := resource("images", "Images", usage, "Daemon-wide image storage can contain shared layers; no image is assigned to one project or selected for cleanup.")
	output, err := i.runner.Run(ctx, executable, "image", "ls", "--all", "--format", "json")
	if err != nil {
		return failedCount(result, report, "images", err)
	}
	ids := make(map[string]struct{})
	for _, line := range nonEmptyLines(output) {
		var value struct {
			ID string `json:"ID"`
		}
		if err := json.Unmarshal([]byte(line), &value); err != nil || value.ID == "" {
			return failedCount(result, report, "images", errors.New("Docker returned a malformed image row"))
		}
		ids[value.ID] = struct{}{}
	}
	result.Count, result.CountAvailable = uint64(len(ids)), true
	return result
}

func (i *Inspector) inspectStoppedContainers(ctx context.Context, executable string, report *core.DockerAwareness) core.DockerResourceSummary {
	result := resource("stopped-containers", "Stopped containers", diskUsageRow{}, "Only non-running container states are counted. Their scoped writable-layer size is not claimed, and no removal action is available.")
	output, err := i.runner.Run(ctx, executable, "container", "ls", "--all", "--filter", "status=created", "--filter", "status=exited", "--filter", "status=dead", "--format", "json")
	if err != nil {
		return failedCount(result, report, "stopped containers", err)
	}
	return setValidatedCount(result, report, "stopped containers", output, "ID")
}

func (i *Inspector) inspectBuildCache(ctx context.Context, executable string, usage diskUsageRow, report *core.DockerAwareness) core.DockerResourceSummary {
	result := resource("build-cache", "BuildKit cache", usage, "The count comes from the active builder while daemon-wide cache bytes can be shared across builders and projects. This phase does not attribute or prune it.")
	output, err := i.runner.Run(ctx, executable, "builder", "du", "--format", "json")
	if err != nil {
		return failedCount(result, report, "BuildKit cache records", err)
	}
	return setValidatedCount(result, report, "BuildKit cache records", output, "ID")
}

func (i *Inspector) inspectNetworks(ctx context.Context, executable string, report *core.DockerAwareness) core.DockerResourceSummary {
	result := resource("networks", "Custom networks", diskUsageRow{}, "Custom networks are reported independently but have no disk-size claim and no removal action.")
	output, err := i.runner.Run(ctx, executable, "network", "ls", "--filter", "type=custom", "--format", "json")
	if err != nil {
		return failedCount(result, report, "custom networks", err)
	}
	return setValidatedCount(result, report, "custom networks", output, "ID")
}

func (i *Inspector) inspectVolumes(ctx context.Context, executable string, usage diskUsageRow, report *core.DockerAwareness) core.DockerResourceSummary {
	result := resource("volumes", "Volumes", usage, "Volumes are persistent-state candidates. They remain Danger scope and cannot be cleaned or mutated in M3.1.")
	result.Stateful = true
	output, err := i.runner.Run(ctx, executable, "volume", "ls", "--format", "json")
	if err != nil {
		return failedCount(result, report, "volumes", err)
	}
	return setValidatedCount(result, report, "volumes", output, "Name")
}

func resource(kind, name string, usage diskUsageRow, boundary string) core.DockerResourceSummary {
	if usage.Size.Kind == "" {
		usage.Size = core.Measurement{Kind: core.MeasurementUnavailable}
	}
	if usage.Reclaimable.Kind == "" {
		usage.Reclaimable = core.Measurement{Kind: core.MeasurementUnavailable}
	}
	return core.DockerResourceSummary{
		Kind:        kind,
		Name:        name,
		Size:        usage.Size,
		Reclaimable: usage.Reclaimable,
		Boundary:    boundary,
	}
}

func setValidatedCount(result core.DockerResourceSummary, report *core.DockerAwareness, label, output, identifier string) core.DockerResourceSummary {
	lines := nonEmptyLines(output)
	for _, line := range lines {
		var value map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			return failedCount(result, report, label, errors.New("Docker returned a malformed JSON row"))
		}
		var id string
		if err := json.Unmarshal(value[identifier], &id); err != nil || id == "" {
			return failedCount(result, report, label, fmt.Errorf("Docker row did not contain %s", identifier))
		}
	}
	result.Count, result.CountAvailable = uint64(len(lines)), true
	return result
}

func failedCount(result core.DockerResourceSummary, report *core.DockerAwareness, label string, err error) core.DockerResourceSummary {
	report.Warnings = append(report.Warnings, fmt.Sprintf("Could not inspect %s: %v", label, err))
	return result
}

func parseSize(value string) core.Measurement {
	value = strings.TrimSpace(value)
	matches := sizePattern.FindStringSubmatch(value)
	if len(matches) != 3 {
		return core.Measurement{Kind: core.MeasurementUnavailable}
	}
	number, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || number < 0 {
		return core.Measurement{Kind: core.MeasurementUnavailable}
	}
	powers := map[string]int{"B": 0, "kB": 1, "MB": 2, "GB": 3, "TB": 4, "PB": 5, "EB": 6}
	power, ok := powers[matches[2]]
	if !ok {
		return core.Measurement{Kind: core.MeasurementUnavailable}
	}
	bytes := number * math.Pow(1000, float64(power))
	if bytes >= math.Pow(2, 64) {
		return core.Measurement{Kind: core.MeasurementUnavailable}
	}
	return core.Measurement{Bytes: uint64(math.Round(bytes)), Kind: core.MeasurementMeasuredLogical}
}

func firstField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func nonEmptyLines(output string) []string {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}
