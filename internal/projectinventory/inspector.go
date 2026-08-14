// Package projectinventory implements M4.1 read-only project discovery below one
// already approved workspace root. It never measures bytes, never issues a cleanup
// plan, and never removes a filesystem path.
package projectinventory

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

// Boundary is the fixed statement rendered beside every discovery result.
const Boundary = "Discovery lists exact marker-backed projects and claimed generated directories below the approved root only. Sizes, reclaim estimates, plans, and deletion are unavailable in this phase."

// Limits bound one discovery pass so an unexpected workspace layout cannot produce
// an unbounded traversal.
type Limits struct {
	MaxDepth       int
	MaxDirectories int
	MaxProjects    int
}

// maxRecordedSkips bounds the recorded skip list so one pathological layout cannot
// produce an unbounded report. Reaching it marks the snapshot incomplete.
const maxRecordedSkips = 500

// DefaultLimits are the fixed M4.1 traversal bounds.
func DefaultLimits() Limits {
	return Limits{MaxDepth: 6, MaxDirectories: 20000, MaxProjects: 500}
}

// markerEcosystems maps an exact-case marker file name to the ecosystem it proves.
// A directory name, extension, or lockfile is never an ecosystem signal.
var markerEcosystems = map[string]core.ProjectEcosystem{
	"package.json":        core.EcosystemNode,
	"Cargo.toml":          core.EcosystemRust,
	"pyproject.toml":      core.EcosystemPython,
	"pom.xml":             core.EcosystemMaven,
	"settings.gradle":     core.EcosystemGradle,
	"settings.gradle.kts": core.EcosystemGradle,
	"build.gradle":        core.EcosystemGradle,
	"build.gradle.kts":    core.EcosystemGradle,
}

type artifactRule struct {
	ecosystem core.ProjectEcosystem
	recovery  core.RecoveryCost
	boundary  string
}

// artifactRules is the approved generated-directory allow-list. A name is reported
// only when one of its rules matches an ecosystem detected in the same project
// directory; the slice order fixes the claiming priority.
var artifactRules = map[string][]artifactRule{
	"node_modules": {{core.EcosystemNode, core.RecoveryDownload, "Installed dependency tree for this project. Removal requires a package-manager install before the project builds again."}},
	"target":       {{core.EcosystemRust, core.RecoveryRebuild, "Cargo build output for this project. Removal requires a full rebuild."}, {core.EcosystemMaven, core.RecoveryRebuild, "Maven build output for this project. Removal requires a rebuild and may re-resolve dependencies."}},
	".venv":        {{core.EcosystemPython, core.RecoveryDownload, "Project virtual environment. Removal requires recreating the environment and reinstalling packages."}},
	".next":        {{core.EcosystemNode, core.RecoveryRebuild, "Next.js build and cache output. Removal requires a rebuild."}},
	"dist":         {{core.EcosystemNode, core.RecoveryRebuild, "Declared JavaScript build output directory. Removal requires a rebuild; confirm no source is kept here before any future action."}},
	"build":        {{core.EcosystemGradle, core.RecoveryRebuild, "Gradle build output for this project. Removal requires a rebuild."}, {core.EcosystemNode, core.RecoveryRebuild, "JavaScript build output directory. Removal requires a rebuild; confirm no source is kept here before any future action."}},
	".gradle":      {{core.EcosystemGradle, core.RecoveryRebuild, "Project-local Gradle state. Removal requires Gradle to rebuild its project metadata. Gradle User Home stays out of scope."}},
	".turbo":       {{core.EcosystemNode, core.RecoveryRebuild, "Turborepo task cache for this project. Removal requires re-running cached tasks."}},
}

// excludedMetadataDirectories are recorded and never traversed.
var excludedMetadataDirectories = map[string]string{
	".git": "Version-control metadata is never traversed or reported as reclaimable storage.",
	".hg":  "Version-control metadata is never traversed or reported as reclaimable storage.",
	".svn": "Version-control metadata is never traversed or reported as reclaimable storage.",
}

// DirectoryReader is the read-only filesystem seam used for traversal. It exposes no
// create, write, rename, or remove operation.
type DirectoryReader interface {
	ReadDir(name string) ([]fs.DirEntry, error)
}

type systemDirectoryReader struct{}

func (systemDirectoryReader) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

// Inspector performs one bounded, read-only discovery pass.
type Inspector struct {
	limits Limits
	reader DirectoryReader
}

// NewSystemInspector builds an inspector with the fixed default limits.
func NewSystemInspector() *Inspector {
	return NewInspector(DefaultLimits())
}

// NewInspector builds an inspector with explicit limits. A non-positive limit falls
// back to the corresponding default so a caller cannot disable a bound.
func NewInspector(limits Limits) *Inspector {
	return NewInspectorWithReader(limits, systemDirectoryReader{})
}

// NewInspectorWithReader builds an inspector with an explicit read-only directory
// source so fail-closed traversal branches can be covered deterministically.
func NewInspectorWithReader(limits Limits, reader DirectoryReader) *Inspector {
	if reader == nil {
		reader = systemDirectoryReader{}
	}
	defaults := DefaultLimits()
	if limits.MaxDepth <= 0 {
		limits.MaxDepth = defaults.MaxDepth
	}
	if limits.MaxDirectories <= 0 {
		limits.MaxDirectories = defaults.MaxDirectories
	}
	if limits.MaxProjects <= 0 {
		limits.MaxProjects = defaults.MaxProjects
	}
	return &Inspector{limits: limits, reader: reader}
}

type walkState struct {
	report      *core.ProjectDiscovery
	root        string
	directories int
	readDirs    int
	stopped     bool
	depthWarned bool
	skipsElided bool
}

// Discover enumerates marker-backed projects below root. The returned report is
// always safe to render: an invalid root, a read failure, or an exhausted bound is
// reported explicitly instead of producing a shorter list that looks complete.
func (i *Inspector) Discover(ctx context.Context, root string) core.ProjectDiscovery {
	report := core.ProjectDiscovery{
		InspectedAt: nowUTC(),
		Complete:    true,
		Projects:    []core.ProjectObservation{},
		Skipped:     []core.ProjectSkippedPath{},
		Boundary:    Boundary,
	}
	if strings.TrimSpace(root) == "" {
		report.Complete = false
		report.Message = "No approved workspace root is configured, so no project was discovered."
		return report
	}
	approved, err := common.ValidateWorkspaceRoot(root)
	if err != nil {
		report.Complete = false
		report.Message = fmt.Sprintf("The workspace root could not be revalidated, so no project was discovered: %v", err)
		return report
	}
	report.Root = approved
	report.RootApproved = true

	state := &walkState{report: &report, root: approved}
	i.walk(ctx, state, approved, 0)
	report.Message = summarize(&report, state)
	return report
}

func (i *Inspector) walk(ctx context.Context, state *walkState, directory string, depth int) {
	if state.stopped {
		return
	}
	if err := ctx.Err(); err != nil {
		state.stopped = true
		state.report.Complete = false
		state.report.Warnings = append(state.report.Warnings, fmt.Sprintf("Project discovery stopped before completing the approved root: %v", err))
		return
	}
	state.directories++
	if state.directories > i.limits.MaxDirectories {
		state.stopped = true
		state.report.Complete = false
		state.report.Truncated = true
		state.report.Warnings = append(state.report.Warnings, fmt.Sprintf("Project discovery stopped after the %d-directory bound; the remaining layout was not inspected.", i.limits.MaxDirectories))
		return
	}

	// Re-check the resolved path immediately before reading it. The parent's cached
	// directory entry can be stale, so a directory that became a reparse point after
	// the descend decision must not be traversed.
	info, statErr := os.Lstat(directory)
	if statErr != nil {
		state.report.Complete = false
		state.recordSkip(directory, core.ProjectSkipUnreadable, fmt.Sprintf("The directory could not be revalidated before reading: %v", statErr))
		state.report.Warnings = append(state.report.Warnings, fmt.Sprintf("%s could not be revalidated, so its contents are unknown rather than empty.", state.relative(directory)))
		return
	}
	if isReparse(info.Mode()) || !info.IsDir() {
		state.recordSkip(directory, core.ProjectSkipReparsePoint, "The path resolved to a reparse point or a non-directory at read time, so it was never traversed.")
		return
	}

	entries, err := i.reader.ReadDir(directory)
	if err != nil {
		state.report.Complete = false
		state.recordSkip(directory, core.ProjectSkipUnreadable, fmt.Sprintf("The directory could not be read: %v", err))
		state.report.Warnings = append(state.report.Warnings, fmt.Sprintf("%s could not be read, so its contents are unknown rather than empty.", state.relative(directory)))
		return
	}
	state.readDirs++
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })

	markers, ecosystems := detectMarkers(entries)
	if len(markers) > 0 {
		if len(state.report.Projects) >= i.limits.MaxProjects {
			state.stopped = true
			state.report.Complete = false
			state.report.Truncated = true
			state.report.Warnings = append(state.report.Warnings, fmt.Sprintf("Project discovery stopped after the %d-project bound; the remaining layout was not inspected.", i.limits.MaxProjects))
			return
		}
		state.report.Projects = append(state.report.Projects, core.ProjectObservation{
			Name:         filepath.Base(directory),
			Path:         directory,
			RelativePath: state.relative(directory),
			Ecosystems:   sortedEcosystems(ecosystems),
			Markers:      markers,
			Artifacts:    collectArtifacts(state, directory, entries, ecosystems),
		})
	}

	for _, entry := range entries {
		if state.stopped {
			return
		}
		name := entry.Name()
		child := filepath.Join(directory, name)
		if isReparse(entry.Type()) {
			state.recordSkip(child, core.ProjectSkipReparsePoint, "A reparse point is recorded but never followed, so its target is not discovered or measured.")
			continue
		}
		if !entry.IsDir() {
			continue
		}
		// Directory-name boundaries are matched case-insensitively because Windows
		// volumes are case-insensitive: a differently cased `.GIT` or `Node_modules`
		// must still be recorded instead of traversed. Marker detection stays
		// exact-case so a mismatch can only claim less, never more.
		if reason, excluded := excludedMetadataDirectories[strings.ToLower(name)]; excluded {
			state.recordSkip(child, core.ProjectSkipExcludedMetadata, reason)
			continue
		}
		if _, generated := artifactRules[strings.ToLower(name)]; generated {
			if _, claimed := claimRule(name, ecosystems); !claimed {
				state.recordSkip(child, core.ProjectSkipUnclaimedName, "The directory name is on the generated allow-list, but no marker in its parent project claims it, so it is not treated as a project artifact.")
			}
			continue
		}
		if depth+1 > i.limits.MaxDepth {
			state.recordSkip(child, core.ProjectSkipDepthLimit, fmt.Sprintf("The fixed %d-level depth bound stopped traversal here.", i.limits.MaxDepth))
			state.report.Truncated = true
			state.report.Complete = false
			if !state.depthWarned {
				state.depthWarned = true
				state.report.Warnings = append(state.report.Warnings, fmt.Sprintf("The fixed %d-level depth bound stopped traversal, so the layout below the recorded paths is unknown.", i.limits.MaxDepth))
			}
			continue
		}
		i.walk(ctx, state, child, depth+1)
	}
}

func detectMarkers(entries []fs.DirEntry) ([]string, map[core.ProjectEcosystem]struct{}) {
	var markers []string
	ecosystems := make(map[core.ProjectEcosystem]struct{})
	for _, entry := range entries {
		if entry.IsDir() || isReparse(entry.Type()) || !entry.Type().IsRegular() {
			continue
		}
		ecosystem, ok := markerEcosystems[entry.Name()]
		if !ok {
			continue
		}
		markers = append(markers, entry.Name())
		ecosystems[ecosystem] = struct{}{}
	}
	sort.Strings(markers)
	return markers, ecosystems
}

func collectArtifacts(state *walkState, directory string, entries []fs.DirEntry, ecosystems map[core.ProjectEcosystem]struct{}) []core.ProjectArtifactObservation {
	artifacts := []core.ProjectArtifactObservation{}
	for _, entry := range entries {
		if !entry.IsDir() || isReparse(entry.Type()) {
			continue
		}
		rule, claimed := claimRule(entry.Name(), ecosystems)
		if !claimed {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		artifacts = append(artifacts, core.ProjectArtifactObservation{
			Name:         entry.Name(),
			Path:         path,
			RelativePath: state.relative(path),
			Ecosystem:    rule.ecosystem,
			StorageClass: core.StorageRebuildable,
			Risk:         core.RiskReview,
			RecoveryCost: rule.recovery,
			Measured:     core.Measurement{Kind: core.MeasurementUnavailable},
			Boundary:     rule.boundary,
		})
	}
	return artifacts
}

func claimRule(name string, ecosystems map[core.ProjectEcosystem]struct{}) (artifactRule, bool) {
	for _, rule := range artifactRules[strings.ToLower(name)] {
		if _, present := ecosystems[rule.ecosystem]; present {
			return rule, true
		}
	}
	return artifactRule{}, false
}

func sortedEcosystems(ecosystems map[core.ProjectEcosystem]struct{}) []core.ProjectEcosystem {
	ordered := make([]core.ProjectEcosystem, 0, len(ecosystems))
	for ecosystem := range ecosystems {
		ordered = append(ordered, ecosystem)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	return ordered
}

// isReparse rejects symbolic links plus the other Windows reparse tags Go reports as
// irregular files, so a junction is never traversed or measured.
func isReparse(mode fs.FileMode) bool {
	return mode&(fs.ModeSymlink|fs.ModeIrregular) != 0
}

func (s *walkState) recordSkip(path string, kind core.ProjectSkipKind, reason string) {
	if len(s.report.Skipped) >= maxRecordedSkips {
		if !s.skipsElided {
			s.skipsElided = true
			s.report.Complete = false
			s.report.Warnings = append(s.report.Warnings, fmt.Sprintf("More than %d paths were recorded as not traversed; the remaining skip records were omitted from this report.", maxRecordedSkips))
		}
		return
	}
	s.report.Skipped = append(s.report.Skipped, core.ProjectSkippedPath{
		RelativePath: s.relative(path),
		Kind:         kind,
		Reason:       reason,
	})
}

func (s *walkState) relative(path string) string {
	relative, err := filepath.Rel(s.root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}

func summarize(report *core.ProjectDiscovery, state *walkState) string {
	artifacts := 0
	for _, project := range report.Projects {
		artifacts += len(project.Artifacts)
	}
	message := fmt.Sprintf("Discovered %d marker-backed project(s) and %d claimed generated director(ies) across %d successfully read director(ies). No bytes were measured and nothing was removed.", len(report.Projects), artifacts, state.readDirs)
	if report.Truncated {
		message += " A traversal bound was reached, so the layout below it is unknown."
	}
	if !report.Complete {
		message += " This snapshot is incomplete; skipped or unreadable paths are not reported as empty."
	}
	return message
}

var nowUTC = func() time.Time {
	return time.Now().UTC()
}
