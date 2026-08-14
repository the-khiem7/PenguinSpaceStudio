package projectinventory

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

// MeasurementBoundary is the fixed statement rendered beside every measurement.
const MeasurementBoundary = "Measured values are exact logical bytes of the regular files that were actually counted, once per path, so a hardlinked file is counted in every artifact that links it. They are not physical host reclaim: hardlinks, sparse files, and cluster size all break that equivalence. Reclaimable bytes remain unavailable, and no cleanup is available in this phase."

// maxMeasuredDepth bounds measurement recursion independently of discovery depth,
// because a dependency tree is legitimately deeper than a project layout.
const maxMeasuredDepth = 64

// cancelCheckInterval bounds how many entries a cancel request can be delayed by
// inside one very large directory listing.
const cancelCheckInterval = 2000

// CancelSignal is a single explicit stop request for one in-flight measurement. Its
// zero value is never cancelled; only Cancel() flips it, and it is safe to poll and
// call Cancel concurrently.
type CancelSignal struct {
	cancelled atomic.Bool
}

func (s *CancelSignal) Cancel() { s.cancelled.Store(true) }

func (s *CancelSignal) cancelledNow() bool {
	if s == nil {
		return false
	}
	return s.cancelled.Load()
}

// MeasureProject measures the claimed generated directories of one project below the
// approved root. The project and its artifacts are re-derived from a fresh discovery
// pass, so a caller cannot supply an arbitrary path. cancel may be nil; when supplied
// and Cancel() is called mid-walk, the walk stops promptly and the partial result is
// returned with Cancelled=true instead of an error, because a deliberate stop is not
// a failure. Nothing is created, modified, or removed.
func (i *Inspector) MeasureProject(ctx context.Context, root, projectPath string, exclusions []string, cancel *CancelSignal) (core.ProjectMeasurement, error) {
	if strings.TrimSpace(projectPath) == "" {
		return core.ProjectMeasurement{}, errors.New("a project path is required")
	}
	discovery := i.Discover(ctx, root)
	if !discovery.RootApproved {
		return core.ProjectMeasurement{}, errors.New(discovery.Message)
	}
	project, found := findProject(discovery, projectPath)
	if !found {
		// An incomplete or truncated snapshot cannot prove that the path is not a
		// project, so the two cases must not share one message.
		if err := ctx.Err(); err != nil {
			return core.ProjectMeasurement{}, fmt.Errorf("the discovery pass behind this measurement did not finish, so %q could not be confirmed as a project: %w", projectPath, err)
		}
		if !discovery.Complete || discovery.Truncated {
			return core.ProjectMeasurement{}, fmt.Errorf("the discovery pass behind this measurement was incomplete, so %q could not be confirmed as a project: %s", projectPath, discovery.Message)
		}
		return core.ProjectMeasurement{}, fmt.Errorf("%q is not a marker-backed project below the approved root; discover projects again before measuring", projectPath)
	}

	rules, err := validateExclusions(discovery.Root, exclusions)
	if err != nil {
		return core.ProjectMeasurement{}, err
	}

	measurement := core.ProjectMeasurement{
		Name:         project.Name,
		Path:         project.Path,
		RelativePath: project.RelativePath,
		Root:         discovery.Root,
		MeasuredAt:   nowUTC(),
		Artifacts:    []core.ProjectArtifactMeasurement{},
		Total:        core.Measurement{Kind: core.MeasurementMeasuredLogical},
		Reclaimable:  core.Measurement{Kind: core.MeasurementUnavailable},
		Complete:     true,
		Exclusions:   exclusionRules(discovery.Root, rules),
		Boundary:     MeasurementBoundary,
	}
	if !discovery.Complete {
		measurement.Complete = false
		measurement.Warnings = append(measurement.Warnings, "The discovery snapshot behind this measurement was incomplete, so this project's artifact list may be missing entries.")
	}
	if discovery.Truncated {
		measurement.Truncated = true
	}

	state := &measureState{root: discovery.Root, rules: rules, cancel: cancel}
	unmeasured := 0
	for _, artifact := range project.Artifacts {
		if state.cancel.cancelledNow() {
			state.cancelledDuringWalk = true
			state.appendWarning("Measurement was cancelled before every claimed artifact could be counted.")
			break
		}
		result := i.measureArtifact(ctx, state, artifact)
		if !result.Complete {
			measurement.Complete = false
		}
		if result.Truncated {
			measurement.Truncated = true
		}
		switch {
		case result.Measured.Kind != core.MeasurementMeasuredLogical:
			// Unknown bytes are never summed as zero.
			unmeasured++
		case math.MaxUint64-measurement.Total.Bytes < result.Measured.Bytes:
			measurement.Complete = false
			measurement.Warnings = append(measurement.Warnings, "The project total overflowed and was not summed; per-artifact values remain exact.")
			measurement.Total = core.Measurement{Kind: core.MeasurementUnavailable}
		case measurement.Total.Kind == core.MeasurementMeasuredLogical:
			measurement.Total.Bytes += result.Measured.Bytes
		}
		measurement.Artifacts = append(measurement.Artifacts, result)
	}
	if unmeasured > 0 {
		measurement.Complete = false
		measurement.Warnings = append(measurement.Warnings, fmt.Sprintf("%d artifact(s) have unknown bytes and were left out of the project total rather than counted as zero.", unmeasured))
	}
	if state.cancelledDuringWalk {
		measurement.Cancelled = true
		measurement.Complete = false
		measurement.Truncated = true
	}
	for index, rule := range measurement.Exclusions {
		if state.matched[rule.Rule] {
			measurement.Exclusions[index].Matched = true
			continue
		}
		measurement.Warnings = append(measurement.Warnings, fmt.Sprintf("Exclusion %q matched nothing in this project, so it removed no bytes from the total.", rule.RelativePath))
	}
	measurement.Warnings = append(measurement.Warnings, state.warnings...)
	measurement.Message = summarizeMeasurement(&measurement)
	return measurement, nil
}

type measureState struct {
	root                string
	rules               []string
	entries             int
	matched             map[string]bool
	warnings            []string
	cancel              *CancelSignal
	cancelledDuringWalk bool
}

func (i *Inspector) measureArtifact(ctx context.Context, state *measureState, artifact core.ProjectArtifactObservation) core.ProjectArtifactMeasurement {
	if state.matched == nil {
		state.matched = make(map[string]bool)
	}
	result := core.ProjectArtifactMeasurement{
		Name:         artifact.Name,
		Path:         artifact.Path,
		RelativePath: artifact.RelativePath,
		Ecosystem:    artifact.Ecosystem,
		StorageClass: artifact.StorageClass,
		Risk:         artifact.Risk,
		RecoveryCost: artifact.RecoveryCost,
		Measured:     core.Measurement{Kind: core.MeasurementMeasuredLogical},
		Reclaimable:  core.Measurement{Kind: core.MeasurementUnavailable},
		Complete:     true,
		Skipped:      []core.ProjectSkippedPath{},
		Boundary:     artifact.Boundary,
	}
	if rule, excluded := state.excludedBy(artifact.Path); excluded {
		result.Complete = false
		// Excluded bytes were never counted, so they are unknown rather than zero.
		result.Measured = core.Measurement{Kind: core.MeasurementUnavailable}
		state.recordSkip(&result, artifact.Path, core.ProjectSkipExcludedByRule,
			fmt.Sprintf("Exclusion %q covers this artifact, so none of its bytes were counted.", state.relative(rule)))
		return result
	}
	i.measureDirectory(ctx, state, &result, artifact.Path, 0)
	if result.Directories == 0 {
		// The artifact root itself was never read, so no byte value exists for it.
		result.Measured = core.Measurement{Kind: core.MeasurementUnavailable}
	}
	return result
}

func (i *Inspector) measureDirectory(ctx context.Context, state *measureState, result *core.ProjectArtifactMeasurement, directory string, depth int) {
	if result.Truncated {
		return
	}
	// An explicit cancel is checked before the context deadline so a cancel that
	// lands at the same instant as the fixed timeout is reported as a deliberate
	// stop rather than as an unrelated deadline expiry.
	if state.cancel.cancelledNow() {
		result.Complete = false
		result.Truncated = true
		state.cancelledDuringWalk = true
		state.appendWarning(fmt.Sprintf("Measurement of %s was cancelled and stopped promptly.", state.relative(directory)))
		return
	}
	if err := ctx.Err(); err != nil {
		result.Complete = false
		result.Truncated = true
		state.appendWarning(fmt.Sprintf("Measurement of %s stopped before completing: %v", state.relative(directory), err))
		return
	}

	// Re-check the resolved path with a no-follow stat so a directory that became a
	// reparse point after its parent listing is never measured.
	info, statErr := os.Lstat(directory)
	if statErr != nil {
		result.Complete = false
		state.recordSkip(result, directory, core.ProjectSkipUnreadable,
			fmt.Sprintf("The directory could not be revalidated before measurement: %v", statErr))
		return
	}
	if isReparse(info.Mode()) || !info.IsDir() {
		result.Complete = false
		state.recordSkip(result, directory, core.ProjectSkipReparsePoint,
			"The path resolved to a reparse point or a non-directory at measurement time, so its bytes were not counted.")
		return
	}

	entries, err := i.reader.ReadDir(directory)
	if err != nil {
		result.Complete = false
		state.recordSkip(result, directory, core.ProjectSkipUnreadable,
			fmt.Sprintf("The directory could not be read, so its bytes are unknown rather than zero: %v", err))
		return
	}
	result.Directories++
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })

	for _, entry := range entries {
		if result.Truncated {
			return
		}
		state.entries++
		// Checked before the entry budget, not only at each directory boundary, so an
		// explicit cancel that lands exactly at the budget is reported as a
		// deliberate stop rather than as an unrelated budget exhaustion, and
		// cancelling mid-listing of one very large directory still stops promptly.
		if state.entries%cancelCheckInterval == 0 && state.cancel.cancelledNow() {
			result.Complete = false
			result.Truncated = true
			state.cancelledDuringWalk = true
			state.appendWarning(fmt.Sprintf("Measurement of %s was cancelled mid-listing and stopped promptly.", state.relative(directory)))
			return
		}
		if state.entries > i.limits.MaxMeasuredEntries {
			result.Complete = false
			result.Truncated = true
			state.appendWarning(fmt.Sprintf("Measurement stopped after the %d-entry budget, so the reported bytes are a partial count rather than a smaller total.", i.limits.MaxMeasuredEntries))
			return
		}
		child := filepath.Join(directory, entry.Name())
		// Safety rules are evaluated before exclusions, so a reparse point is always
		// recorded as one and an exclusion can never re-include it.
		if isReparse(entry.Type()) {
			result.Complete = false
			state.recordSkip(result, child, core.ProjectSkipReparsePoint,
				"A reparse point is never followed, so the storage it points at was not counted here.")
			continue
		}
		if rule, excluded := state.excludedBy(child); excluded {
			result.Complete = false
			state.recordSkip(result, child, core.ProjectSkipExcludedByRule,
				fmt.Sprintf("Exclusion %q covers this path, so its bytes were not counted.", state.relative(rule)))
			continue
		}
		if entry.IsDir() {
			if depth+1 > maxMeasuredDepth {
				result.Complete = false
				result.Truncated = true
				state.recordSkip(result, child, core.ProjectSkipDepthLimit,
					fmt.Sprintf("The fixed %d-level measurement depth bound stopped counting here.", maxMeasuredDepth))
				state.appendWarning(fmt.Sprintf("Measurement stopped at the %d-level depth bound, so the reported bytes are a partial count.", maxMeasuredDepth))
				continue
			}
			i.measureDirectory(ctx, state, result, child, depth+1)
			continue
		}
		if !entry.Type().IsRegular() {
			result.Complete = false
			state.recordSkip(result, child, core.ProjectSkipNonRegular,
				"Only regular files contribute logical bytes, so this entry was recorded instead of counted.")
			continue
		}
		fileInfo, infoErr := entry.Info()
		if infoErr != nil {
			result.Complete = false
			state.recordSkip(result, child, core.ProjectSkipUnreadable,
				fmt.Sprintf("The file size could not be read, so it is unknown rather than zero: %v", infoErr))
			continue
		}
		if !fileInfo.Mode().IsRegular() || fileInfo.Size() < 0 {
			result.Complete = false
			state.recordSkip(result, child, core.ProjectSkipNonRegular,
				"The entry stopped being a countable regular file before its size was read.")
			continue
		}
		size := uint64(fileInfo.Size())
		if math.MaxUint64-result.Measured.Bytes < size {
			result.Complete = false
			result.Truncated = true
			state.appendWarning(fmt.Sprintf("Measurement of %s overflowed and stopped; the reported bytes are partial.", result.RelativePath))
			return
		}
		result.Measured.Bytes += size
		result.Files++
	}
}

// excludedBy reports the first validated rule that covers path and marks every rule
// that covers it, so an overlapping narrower rule is not reported as unmatched. A rule
// covers the exact path and everything below it.
func (s *measureState) excludedBy(path string) (string, bool) {
	for _, rule := range s.rules {
		if !isWithin(rule, path) {
			continue
		}
		s.matched[rule] = true
		for _, other := range s.rules {
			if other != rule && isWithin(rule, other) {
				s.matched[other] = true
			}
		}
		return rule, true
	}
	return "", false
}

// recordSkip appends a measurement skip, bounded so one pathological artifact cannot
// produce an unbounded report.
func (s *measureState) recordSkip(result *core.ProjectArtifactMeasurement, path string, kind core.ProjectSkipKind, reason string) {
	if len(result.Skipped) >= maxRecordedSkips {
		s.appendWarning(fmt.Sprintf("More than %d measurement skips were recorded for one artifact; the remaining records were omitted.", maxRecordedSkips))
		return
	}
	result.Skipped = append(result.Skipped, core.ProjectSkippedPath{
		RelativePath: s.relative(path),
		Kind:         kind,
		Reason:       reason,
	})
}

func (s *measureState) appendWarning(warning string) {
	for _, existing := range s.warnings {
		if existing == warning {
			return
		}
	}
	s.warnings = append(s.warnings, warning)
}

func (s *measureState) relative(path string) string {
	relative, err := filepath.Rel(s.root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}

// validateExclusions converts caller-proposed paths into absolute rules inside the
// approved root. A rule that is not absolute is resolved against the root; every rule
// is rejected if it escapes the root, targets the root itself, or passes through a
// symbolic link.
func validateExclusions(root string, exclusions []string) ([]string, error) {
	rules := make([]string, 0, len(exclusions))
	seen := make(map[string]struct{})
	for _, exclusion := range exclusions {
		candidate := strings.TrimSpace(exclusion)
		if candidate == "" {
			continue
		}
		if strings.ContainsAny(candidate, "*?[") {
			return nil, fmt.Errorf("exclusion %q uses a pattern; only exact paths below the approved root are accepted", exclusion)
		}
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(root, candidate)
		}
		rule, err := common.ValidateWorkspaceTarget(root, candidate, "exclusion")
		if err != nil {
			return nil, fmt.Errorf("exclusion %q was rejected: %w", exclusion, err)
		}
		key := strings.ToLower(filepath.Clean(rule))
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		rules = append(rules, rule)
	}
	return rules, nil
}

func exclusionRules(root string, rules []string) []core.ProjectExclusionRule {
	reported := make([]core.ProjectExclusionRule, 0, len(rules))
	for _, rule := range rules {
		relative, err := filepath.Rel(root, rule)
		if err != nil {
			relative = rule
		}
		reported = append(reported, core.ProjectExclusionRule{Rule: rule, RelativePath: filepath.ToSlash(relative)})
	}
	return reported
}

func findProject(discovery core.ProjectDiscovery, projectPath string) (core.ProjectObservation, bool) {
	for _, project := range discovery.Projects {
		if common.SamePath(project.Path, projectPath) {
			return project, true
		}
	}
	return core.ProjectObservation{}, false
}

// isWithin reports whether path is the ancestor path itself or sits below it. It
// compares cleaned path components, so a sibling with a shared name prefix such as
// `build-output` is never treated as being inside `build`.
func isWithin(ancestor, path string) bool {
	if common.SamePath(ancestor, path) {
		return true
	}
	relative, err := filepath.Rel(filepath.Clean(ancestor), filepath.Clean(path))
	if err != nil {
		return false
	}
	if relative == "." {
		return true
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func summarizeMeasurement(measurement *core.ProjectMeasurement) string {
	var files, directories uint64
	excluded, skipped := 0, 0
	for _, artifact := range measurement.Artifacts {
		files += artifact.Files
		directories += artifact.Directories
		for _, skip := range artifact.Skipped {
			if skip.Kind == core.ProjectSkipExcludedByRule {
				excluded++
				continue
			}
			skipped++
		}
	}
	message := fmt.Sprintf("Counted %d regular file(s) across %d director(ies) in %d claimed artifact(s). Reclaimable bytes remain unavailable and nothing was removed.", files, directories, len(measurement.Artifacts))
	if excluded > 0 {
		message += fmt.Sprintf(" %d path(s) were excluded by rule, so the real size is larger than the measured value.", excluded)
	}
	if skipped > 0 {
		message += fmt.Sprintf(" %d path(s) were skipped for safety and their bytes are unknown rather than zero.", skipped)
	}
	if measurement.Cancelled {
		message += " Measurement was cancelled; the reported bytes are a partial count gathered before the stop."
	} else if measurement.Truncated {
		message += " A measurement budget was reached, so the reported bytes are a partial count."
	}
	if !measurement.Complete {
		message += " This measurement is scoped, not the full size of the artifacts."
	}
	return message
}
