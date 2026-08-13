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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

var (
	sizePattern           = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)([kMGTPE]?B)$`)
	composeProjectPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

const inspectBatchSize = 48

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
	report.Daemon.Message = "Docker daemon is available. Resource inspection is read-only; eligible Compose network removal requires a separate reviewed plan."

	diskUsage := i.inspectDiskUsage(ctx, executable, &report)
	buildCache, builder := i.inspectBuildCache(ctx, executable, diskUsage["Build Cache"], &report)
	report.Resources = []core.DockerResourceSummary{
		i.inspectImages(ctx, executable, diskUsage["Images"], &report),
		i.inspectStoppedContainers(ctx, executable, &report),
		buildCache,
		i.inspectNetworks(ctx, executable, &report),
		i.inspectVolumes(ctx, executable, diskUsage["Local Volumes"], &report),
	}
	report.OwnershipGroups, report.OwnershipComplete = i.inspectOwnership(ctx, executable, &report)
	report.Builder = builder
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
	result := resource("images", "Images", usage, "Daemon-wide image storage can contain shared layers. Compose labels are shown only as grouping metadata and never as cleanup authority.")
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
	result := resource("stopped-containers", "Stopped containers", diskUsageRow{}, "Only created, exited, or dead containers are grouped. Their writable-layer size is not claimed, and no removal action is available.")
	output, err := i.runner.Run(ctx, executable, "container", "ls", "--all", "--filter", "status=created", "--filter", "status=exited", "--filter", "status=dead", "--format", "json")
	if err != nil {
		return failedCount(result, report, "stopped containers", err)
	}
	return setValidatedCount(result, report, "stopped containers", output, "ID")
}

func (i *Inspector) inspectBuildCache(ctx context.Context, executable string, usage diskUsageRow, report *core.DockerAwareness) (core.DockerResourceSummary, core.DockerBuilderScope) {
	result := resource("build-cache", "BuildKit cache", usage, "Records belong to the selected builder scope. They are not attributed to Compose projects and no prune action is available.")
	scope := core.DockerBuilderScope{
		Scope:    "selected-builder",
		Name:     "Selected Docker builder",
		Boundary: "BuildKit records are reported by docker builder du for the selected builder only. Shared records are not project ownership evidence.",
	}
	output, err := i.runner.Run(ctx, executable, "builder", "du", "--format", "json")
	if err != nil {
		return failedCount(result, report, "BuildKit cache records", err), scope
	}
	for _, line := range nonEmptyLines(output) {
		var value struct {
			ID          string `json:"ID"`
			Shared      bool   `json:"Shared"`
			Mutable     bool   `json:"Mutable"`
			Reclaimable bool   `json:"Reclaimable"`
		}
		if err := json.Unmarshal([]byte(line), &value); err != nil || value.ID == "" {
			return failedCount(result, report, "BuildKit cache records", errors.New("Docker returned a malformed BuildKit row")), scope
		}
		scope.Records = append(scope.Records, core.DockerBuildCacheRecord{
			ID: value.ID, Shared: value.Shared, Mutable: value.Mutable, Reclaimable: value.Reclaimable,
		})
		if value.Shared {
			scope.SharedCount++
		}
	}
	scope.Count = uint64(len(scope.Records))
	scope.CountAvailable = true
	result.Count, result.CountAvailable = scope.Count, true
	return result, scope
}

func (i *Inspector) inspectNetworks(ctx context.Context, executable string, report *core.DockerAwareness) core.DockerResourceSummary {
	result := resource("networks", "Custom networks", diskUsageRow{}, "Custom networks are grouped by exact Compose labels. Only one complete-snapshot, canonically labeled, unattached network can proceed to a separate exact-ID Review plan.")
	output, err := i.runner.Run(ctx, executable, "network", "ls", "--filter", "type=custom", "--format", "json")
	if err != nil {
		return failedCount(result, report, "custom networks", err)
	}
	return setValidatedCount(result, report, "custom networks", output, "ID")
}

func (i *Inspector) inspectVolumes(ctx context.Context, executable string, usage diskUsageRow, report *core.DockerAwareness) core.DockerResourceSummary {
	result := resource("volumes", "Volumes", usage, "Volumes remain Stateful and Danger regardless of Compose labels or current mount count. M3.3 cannot clean or mutate them.")
	result.Stateful = true
	output, err := i.runner.Run(ctx, executable, "volume", "ls", "--format", "json")
	if err != nil {
		return failedCount(result, report, "volumes", err)
	}
	return setValidatedCount(result, report, "volumes", output, "Name")
}

type imageInspect struct {
	ID       string `json:"Id"`
	RepoTags []string
	Config   struct {
		Labels map[string]string
	}
}

type containerInspect struct {
	ID    string `json:"Id"`
	Name  string
	Image string
	State struct {
		Status string
	}
	Config struct {
		Labels map[string]string
	}
	NetworkSettings struct {
		Networks map[string]json.RawMessage
	}
	Mounts []struct {
		Type string
		Name string
	}
}

type networkInspect struct {
	ID         string `json:"Id"`
	Name       string
	Labels     map[string]string
	Containers *map[string]json.RawMessage
}

type volumeInspect struct {
	Name   string
	Labels map[string]string
}

func (i *Inspector) inspectOwnership(ctx context.Context, executable string, report *core.DockerAwareness) ([]core.DockerOwnershipGroup, bool) {
	imageIDs, imagesListed := i.listIdentifiers(ctx, executable, report, "images for ownership", "ID", "image", "ls", "--all", "--no-trunc", "--format", "json")
	containerIDs, containersListed := i.listIdentifiers(ctx, executable, report, "containers for relationships", "ID", "container", "ls", "--all", "--no-trunc", "--format", "json")
	networkIDs, networksListed := i.listIdentifiers(ctx, executable, report, "custom networks for ownership", "ID", "network", "ls", "--no-trunc", "--filter", "type=custom", "--format", "json")
	volumeNames, volumesListed := i.listIdentifiers(ctx, executable, report, "volumes for ownership", "Name", "volume", "ls", "--format", "json")

	imageRows, imagesInspected := i.inspectRows(ctx, executable, report, "images for ownership", []string{"image", "inspect"}, imageIDs, "Id")
	containerRows, containersInspected := i.inspectRows(ctx, executable, report, "containers for relationships", []string{"container", "inspect"}, containerIDs, "Id")
	networkRows, networksInspected := i.inspectRows(ctx, executable, report, "custom networks for ownership", []string{"network", "inspect"}, networkIDs, "Id")
	volumeRows, volumesInspected := i.inspectRows(ctx, executable, report, "volumes for ownership", []string{"volume", "inspect"}, volumeNames, "Name")
	containerRelationshipsAvailable := containersListed && containersInspected
	ownershipComplete := imagesListed && imagesInspected && containersListed && containersInspected && networksListed && networksInspected && volumesListed && volumesInspected

	containers := make([]containerInspect, 0, len(containerRows))
	imageReferences := make(map[string]uint64)
	volumeMounts := make(map[string]uint64)
	for _, row := range containerRows {
		var value containerInspect
		if err := json.Unmarshal(row, &value); err != nil || value.ID == "" {
			containerRelationshipsAvailable = false
			ownershipComplete = false
			report.Warnings = append(report.Warnings, "Docker returned a malformed container inspect row; dependent relationship counts are unavailable.")
			continue
		}
		containers = append(containers, value)
		if value.Image != "" {
			imageReferences[value.Image]++
		}
		for _, mount := range value.Mounts {
			if mount.Type == "volume" && mount.Name != "" {
				volumeMounts[mount.Name]++
			}
		}
	}

	observations := make([]core.DockerScopedResource, 0, len(imageRows)+len(containers)+len(networkRows)+len(volumeRows))
	for _, row := range imageRows {
		var value imageInspect
		if err := json.Unmarshal(row, &value); err != nil || value.ID == "" {
			ownershipComplete = false
			report.Warnings = append(report.Warnings, "Docker returned a malformed image inspect row; that image was not grouped.")
			continue
		}
		name := shortID(value.ID)
		if len(value.RepoTags) > 0 && value.RepoTags[0] != "<none>:<none>" {
			name = value.RepoTags[0]
		}
		observations = append(observations, scopedResource(value.ID, "image", name, value.Config.Labels, false, core.RiskReview,
			relationship("container-references", imageReferences[value.ID], containerRelationshipsAvailable), ""))
	}
	for _, value := range containers {
		if !isStoppedState(value.State.Status) {
			continue
		}
		observations = append(observations, scopedResource(value.ID, "stopped-container", strings.TrimPrefix(value.Name, "/"), value.Config.Labels, false, core.RiskReview,
			append(
				relationship("networks", uint64(len(value.NetworkSettings.Networks)), true),
				relationship("mounts", uint64(len(value.Mounts)), true)...,
			), value.Image))
	}
	for _, row := range networkRows {
		var value networkInspect
		if err := json.Unmarshal(row, &value); err != nil || value.ID == "" {
			ownershipComplete = false
			report.Warnings = append(report.Warnings, "Docker returned a malformed network inspect row; that network was not grouped.")
			continue
		}
		attachmentsAvailable := value.Containers != nil
		var attachmentCount uint64
		if attachmentsAvailable {
			attachmentCount = uint64(len(*value.Containers))
		} else {
			ownershipComplete = false
			report.Warnings = append(report.Warnings, "Docker omitted network attachment metadata; the relationship is unavailable and ownership is incomplete.")
		}
		observations = append(observations, scopedResource(value.ID, "network", value.Name, value.Labels, false, core.RiskReview,
			relationship("container-attachments", attachmentCount, attachmentsAvailable), ""))
	}
	for _, row := range volumeRows {
		var value volumeInspect
		if err := json.Unmarshal(row, &value); err != nil || value.Name == "" {
			ownershipComplete = false
			report.Warnings = append(report.Warnings, "Docker returned a malformed volume inspect row; that volume was not grouped.")
			continue
		}
		observations = append(observations, scopedResource(value.Name, "volume", value.Name, value.Labels, true, core.RiskDanger,
			relationship("container-mounts", volumeMounts[value.Name], containerRelationshipsAvailable), ""))
	}
	return groupOwnership(observations), ownershipComplete
}

func (i *Inspector) listIdentifiers(ctx context.Context, executable string, report *core.DockerAwareness, label, field string, args ...string) ([]string, bool) {
	output, err := i.runner.Run(ctx, executable, args...)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Could not inspect %s: %v", label, err))
		return nil, false
	}
	seen := make(map[string]struct{})
	values := make([]string, 0)
	for _, line := range nonEmptyLines(output) {
		var row map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Docker returned malformed JSON while listing %s.", label))
			return values, false
		}
		var value string
		if err := json.Unmarshal(row[field], &value); err != nil || value == "" {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Docker did not return an identifier while listing %s.", label))
			return values, false
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			values = append(values, value)
		}
	}
	return values, true
}

func (i *Inspector) inspectRows(ctx context.Context, executable string, report *core.DockerAwareness, label string, command []string, identifiers []string, identityField string) ([]json.RawMessage, bool) {
	rows := make([]json.RawMessage, 0, len(identifiers))
	expected := make(map[string]struct{}, len(identifiers))
	for _, identifier := range identifiers {
		expected[identifier] = struct{}{}
	}
	seen := make(map[string]struct{}, len(identifiers))
	complete := true
	for start := 0; start < len(identifiers); start += inspectBatchSize {
		end := min(start+inspectBatchSize, len(identifiers))
		args := append(append([]string{}, command...), "--format", "{{json .}}")
		args = append(args, identifiers[start:end]...)
		output, err := i.runner.Run(ctx, executable, args...)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Could not inspect %s: %v", label, err))
			complete = false
			continue
		}
		for _, line := range nonEmptyLines(output) {
			var value map[string]json.RawMessage
			if err := json.Unmarshal([]byte(line), &value); err != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("Docker returned malformed inspect data for %s.", label))
				complete = false
				continue
			}
			var identity string
			if err := json.Unmarshal(value[identityField], &identity); err != nil || identity == "" {
				report.Warnings = append(report.Warnings, fmt.Sprintf("Docker returned inspect data without an identity for %s.", label))
				complete = false
				continue
			}
			if _, requested := expected[identity]; !requested {
				report.Warnings = append(report.Warnings, fmt.Sprintf("Docker returned an unexpected inspect identity for %s.", label))
				complete = false
				continue
			}
			if _, duplicate := seen[identity]; duplicate {
				report.Warnings = append(report.Warnings, fmt.Sprintf("Docker returned a duplicate inspect identity for %s.", label))
				complete = false
				continue
			}
			seen[identity] = struct{}{}
			rows = append(rows, json.RawMessage(line))
		}
	}
	if len(seen) != len(expected) {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Docker did not return one inspect row for every requested identity in %s.", label))
		complete = false
	}
	return rows, complete
}

func scopedResource(id, kind, name string, rawLabels map[string]string, stateful bool, risk core.RiskLevel, relationships []core.DockerRelationshipObservation, relatedID string) core.DockerScopedResource {
	labels := composeLabels(rawLabels)
	scope := "unscoped"
	if validComposeScope(kind, labels) {
		scope = "compose-project"
	}
	return core.DockerScopedResource{
		ID: id, Kind: kind, Name: name, Scope: scope, Labels: labels, Relationships: relationships,
		RelatedResourceID: relatedID, Stateful: stateful, Risk: risk,
	}
}

func validComposeScope(kind string, labels core.DockerComposeLabels) bool {
	if !composeProjectPattern.MatchString(labels.Project) {
		return false
	}
	for _, value := range []string{labels.Service, labels.Network, labels.Volume} {
		if value != "" && strings.TrimSpace(value) != value {
			return false
		}
	}
	switch kind {
	case "image", "stopped-container":
		return labels.Network == "" && labels.Volume == ""
	case "network":
		return labels.Service == "" && labels.Volume == ""
	case "volume":
		return labels.Service == "" && labels.Network == ""
	default:
		return false
	}
}

func composeLabels(labels map[string]string) core.DockerComposeLabels {
	return core.DockerComposeLabels{
		Project: labels["com.docker.compose.project"],
		Service: labels["com.docker.compose.service"],
		Network: labels["com.docker.compose.network"],
		Volume:  labels["com.docker.compose.volume"],
	}
}

func relationship(kind string, count uint64, available bool) []core.DockerRelationshipObservation {
	return []core.DockerRelationshipObservation{{Kind: kind, Count: count, Available: available}}
}

func groupOwnership(resources []core.DockerScopedResource) []core.DockerOwnershipGroup {
	projects := make(map[string][]core.DockerScopedResource)
	unscoped := make([]core.DockerScopedResource, 0)
	for _, item := range resources {
		if item.Scope == "compose-project" {
			projects[item.Labels.Project] = append(projects[item.Labels.Project], item)
		} else {
			unscoped = append(unscoped, item)
		}
	}
	projectNames := make([]string, 0, len(projects))
	for project := range projects {
		projectNames = append(projectNames, project)
	}
	sort.Strings(projectNames)
	groups := make([]core.DockerOwnershipGroup, 0, len(projectNames)+1)
	for _, project := range projectNames {
		items := projects[project]
		sortScopedResources(items)
		groups = append(groups, core.DockerOwnershipGroup{Scope: "compose-project", Project: project, Resources: items})
	}
	sortScopedResources(unscoped)
	groups = append(groups, core.DockerOwnershipGroup{Scope: "unscoped", Resources: unscoped})
	return groups
}

func sortScopedResources(resources []core.DockerScopedResource) {
	sort.Slice(resources, func(left, right int) bool {
		if resources[left].Kind == resources[right].Kind {
			return resources[left].Name < resources[right].Name
		}
		return resources[left].Kind < resources[right].Kind
	})
}

func isStoppedState(state string) bool {
	return state == "created" || state == "exited" || state == "dead"
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
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
