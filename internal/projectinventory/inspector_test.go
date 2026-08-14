package projectinventory

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

func writeFixture(t *testing.T, root string, directories []string, files []string) {
	t.Helper()
	for _, directory := range directories {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(directory)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func projectByRelativePath(report core.ProjectDiscovery, relative string) (core.ProjectObservation, bool) {
	for _, project := range report.Projects {
		if project.RelativePath == relative {
			return project, true
		}
	}
	return core.ProjectObservation{}, false
}

func artifactNames(project core.ProjectObservation) []string {
	names := make([]string, 0, len(project.Artifacts))
	for _, artifact := range project.Artifacts {
		names = append(names, artifact.Name)
	}
	return names
}

func skipKind(report core.ProjectDiscovery, relative string) (core.ProjectSkipKind, bool) {
	for _, skip := range report.Skipped {
		if skip.RelativePath == relative {
			return skip.Kind, true
		}
	}
	return "", false
}

func TestUnavailableLastModifiedOmitsValueFromJSON(t *testing.T) {
	encoded, err := json.Marshal(core.TimeObservation{Available: false})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "\"value\"") {
		t.Fatalf("an unavailable observation must never serialize a value field: %s", encoded)
	}
	if string(encoded) != `{"available":false}` {
		t.Fatalf("unexpected encoding: %s", encoded)
	}
}

func TestDiscoverReportsLastModifiedAsDirectoryModTimeOnly(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"dist"}, []string{"package.json"})
	distPath := filepath.Join(root, "dist")

	// Set the directory's own mtime to an exact known value and prove discovery
	// reports exactly that value, not the current time and not a file inside it.
	stamp := time.Date(2024, 3, 15, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(distPath, stamp, stamp); err != nil {
		t.Fatal(err)
	}

	report := NewSystemInspector().Discover(context.Background(), root)
	project, ok := projectByRelativePath(report, ".")
	if !ok {
		t.Fatal("project was not discovered")
	}
	if !project.LastModified.Available {
		t.Fatalf("project LastModified must be available: %+v", project.LastModified)
	}

	var dist core.ProjectArtifactObservation
	for _, artifact := range project.Artifacts {
		if artifact.Name == "dist" {
			dist = artifact
		}
	}
	if dist.Name == "" {
		t.Fatal("dist artifact was not reported")
	}
	if !dist.LastModified.Available {
		t.Fatalf("artifact LastModified must be available: %+v", dist.LastModified)
	}
	if dist.LastModified.Value == nil || !dist.LastModified.Value.Equal(stamp) {
		t.Fatalf("expected exact directory mtime %v, got %v", stamp, dist.LastModified.Value)
	}
}

func TestDiscoverReportsUnavailableLastModifiedForReparseArtifact(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, root, nil, []string{"Cargo.toml"})
	if err := os.Symlink(outside, filepath.Join(root, "target")); err != nil {
		t.Skipf("symlink unsupported on this host: %v", err)
	}

	report := NewSystemInspector().Discover(context.Background(), root)
	project, ok := projectByRelativePath(report, ".")
	if !ok {
		t.Fatal("project was not discovered")
	}
	if len(project.Artifacts) != 0 {
		t.Fatalf("a reparse point must not be reported as a claimed artifact: %+v", project.Artifacts)
	}
	if kind, ok := skipKind(report, "target"); !ok || kind != core.ProjectSkipReparsePoint {
		t.Fatalf("expected the reparse point to be recorded, got %q (present=%v)", kind, ok)
	}
}

func TestDiscoverReportsMarkerBackedProjectsAndClaimedArtifacts(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root,
		[]string{
			"node_modules/left", "dist", ".turbo",
			"services/api/target", "services/api/src",
			"services/jvm/build", "services/jvm/.gradle",
			"services/py/.venv",
			"services/legacy/target",
		},
		[]string{
			"package.json",
			"services/api/Cargo.toml",
			"services/jvm/settings.gradle", "services/jvm/build.gradle",
			"services/py/pyproject.toml",
			"services/legacy/readme.md",
		},
	)

	report := NewSystemInspector().Discover(context.Background(), root)
	if !report.RootApproved || !report.Complete || report.Truncated {
		t.Fatalf("unexpected snapshot state: %+v", report)
	}
	if len(report.Projects) != 4 {
		t.Fatalf("expected 4 marker-backed projects, got %d (%+v)", len(report.Projects), report.Projects)
	}

	node, ok := projectByRelativePath(report, ".")
	if !ok {
		t.Fatal("approved root was not reported as a node project")
	}
	if got := artifactNames(node); len(got) != 3 || got[0] != ".turbo" || got[1] != "dist" || got[2] != "node_modules" {
		t.Fatalf("unexpected node artifacts: %v", got)
	}
	for _, artifact := range node.Artifacts {
		if artifact.Measured.Kind != core.MeasurementUnavailable || artifact.Measured.Bytes != 0 {
			t.Fatalf("M4.1 must not measure bytes: %+v", artifact.Measured)
		}
		if artifact.Risk != core.RiskReview || artifact.StorageClass != core.StorageRebuildable {
			t.Fatalf("unexpected classification: %+v", artifact)
		}
	}
	if node.Artifacts[2].RecoveryCost != core.RecoveryDownload {
		t.Fatalf("node_modules must be Download recovery, got %q", node.Artifacts[2].RecoveryCost)
	}

	rust, ok := projectByRelativePath(report, "services/api")
	if !ok {
		t.Fatal("nested Cargo project was not discovered")
	}
	if got := artifactNames(rust); len(got) != 1 || got[0] != "target" {
		t.Fatalf("unexpected Cargo artifacts: %v", got)
	}
	if rust.Artifacts[0].Ecosystem != core.EcosystemRust {
		t.Fatalf("target must be claimed by rust here, got %q", rust.Artifacts[0].Ecosystem)
	}

	gradle, ok := projectByRelativePath(report, "services/jvm")
	if !ok {
		t.Fatal("nested Gradle project was not discovered")
	}
	if got := artifactNames(gradle); len(got) != 2 || got[0] != ".gradle" || got[1] != "build" {
		t.Fatalf("unexpected Gradle artifacts: %v", got)
	}
	if gradle.Artifacts[1].Ecosystem != core.EcosystemGradle {
		t.Fatalf("build must be claimed by gradle here, got %q", gradle.Artifacts[1].Ecosystem)
	}

	python, ok := projectByRelativePath(report, "services/py")
	if !ok {
		t.Fatal("nested pyproject project was not discovered")
	}
	if got := artifactNames(python); len(got) != 1 || got[0] != ".venv" {
		t.Fatalf("unexpected Python artifacts: %v", got)
	}
	if python.Artifacts[0].RecoveryCost != core.RecoveryDownload {
		t.Fatalf(".venv must be Download recovery, got %q", python.Artifacts[0].RecoveryCost)
	}
}

func TestDiscoverRecordsUnclaimedGeneratedNameWithoutTraversal(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root,
		[]string{"orphan/node_modules/deep/deeper", "orphan/target"},
		[]string{"package.json", "orphan/node_modules/deep/deeper/package.json"},
	)

	report := NewSystemInspector().Discover(context.Background(), root)
	for _, project := range report.Projects {
		if project.RelativePath != "." {
			t.Fatalf("no project may be discovered inside an unclaimed generated directory: %q", project.RelativePath)
		}
	}
	kind, ok := skipKind(report, "orphan/node_modules")
	if !ok || kind != core.ProjectSkipUnclaimedName {
		t.Fatalf("expected unclaimed-generated-name skip, got %q (present=%v)", kind, ok)
	}
	if kind, ok := skipKind(report, "orphan/target"); !ok || kind != core.ProjectSkipUnclaimedName {
		t.Fatalf("expected unclaimed target skip, got %q (present=%v)", kind, ok)
	}
	if !report.Complete {
		t.Fatal("an unclaimed generated name is a recorded boundary, not an incomplete snapshot")
	}
}

func TestDiscoverDoesNotTraverseClaimedArtifactOrVCSMetadata(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root,
		[]string{"node_modules/nested", ".git/objects"},
		[]string{"package.json", "node_modules/nested/Cargo.toml", ".git/objects/package.json"},
	)

	report := NewSystemInspector().Discover(context.Background(), root)
	if len(report.Projects) != 1 {
		t.Fatalf("expected only the root project, got %+v", report.Projects)
	}
	if kind, ok := skipKind(report, ".git"); !ok || kind != core.ProjectSkipExcludedMetadata {
		t.Fatalf("expected excluded-metadata skip for .git, got %q (present=%v)", kind, ok)
	}
	if _, ok := skipKind(report, "node_modules"); ok {
		t.Fatal("a claimed artifact is reported as an artifact, not as a skip")
	}
}

func TestDiscoverRecordsReparsePointWithoutFollowingIt(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, root, nil, []string{"package.json"})
	writeFixture(t, outside, []string{"target"}, []string{"Cargo.toml"})
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink unsupported on this host: %v", err)
	}

	report := NewSystemInspector().Discover(context.Background(), root)
	if len(report.Projects) != 1 {
		t.Fatalf("a reparse point must not contribute projects: %+v", report.Projects)
	}
	if kind, ok := skipKind(report, "linked"); !ok || kind != core.ProjectSkipReparsePoint {
		t.Fatalf("expected reparse-point skip, got %q (present=%v)", kind, ok)
	}
}

func TestDiscoverDoesNotClaimOtherEcosystemArtifactNames(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"dist", ".venv"}, []string{"Cargo.toml"})

	report := NewSystemInspector().Discover(context.Background(), root)
	project, ok := projectByRelativePath(report, ".")
	if !ok {
		t.Fatal("Cargo project was not discovered")
	}
	if len(project.Artifacts) != 0 {
		t.Fatalf("rust markers must not claim dist or .venv: %v", artifactNames(project))
	}
	if kind, ok := skipKind(report, "dist"); !ok || kind != core.ProjectSkipUnclaimedName {
		t.Fatalf("expected dist to be recorded unclaimed, got %q (present=%v)", kind, ok)
	}
}

func TestDiscoverEnforcesDepthBound(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"a/b/c"}, []string{"a/b/c/package.json"})

	report := NewInspector(Limits{MaxDepth: 1}).Discover(context.Background(), root)
	if len(report.Projects) != 0 {
		t.Fatalf("depth bound must stop discovery: %+v", report.Projects)
	}
	if !report.Truncated {
		t.Fatal("depth bound must mark the snapshot truncated")
	}
	if kind, ok := skipKind(report, "a/b"); !ok || kind != core.ProjectSkipDepthLimit {
		t.Fatalf("expected depth-limit skip, got %q (present=%v)", kind, ok)
	}
}

func TestDiscoverEnforcesDirectoryAndProjectBounds(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"one", "two"}, []string{"one/package.json", "two/package.json"})

	directoryBounded := NewInspector(Limits{MaxDirectories: 1}).Discover(context.Background(), root)
	if directoryBounded.Complete || !directoryBounded.Truncated || len(directoryBounded.Warnings) == 0 {
		t.Fatalf("directory bound must fail closed: %+v", directoryBounded)
	}

	projectBounded := NewInspector(Limits{MaxProjects: 1}).Discover(context.Background(), root)
	if len(projectBounded.Projects) != 1 || projectBounded.Complete || !projectBounded.Truncated {
		t.Fatalf("project bound must fail closed: %+v", projectBounded)
	}
}

func TestDiscoverRejectsUnapprovedRoots(t *testing.T) {
	empty := NewSystemInspector().Discover(context.Background(), "  ")
	if empty.RootApproved || empty.Complete || empty.Message == "" {
		t.Fatalf("blank root must be rejected: %+v", empty)
	}

	missing := NewSystemInspector().Discover(context.Background(), filepath.Join(t.TempDir(), "absent"))
	if missing.RootApproved || missing.Complete {
		t.Fatalf("missing root must be rejected: %+v", missing)
	}

	volumeRoot := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	if broad := NewSystemInspector().Discover(context.Background(), volumeRoot); broad.RootApproved {
		t.Fatal("filesystem root must never be approved for discovery")
	}

	file := filepath.Join(t.TempDir(), "package.json")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if regular := NewSystemInspector().Discover(context.Background(), file); regular.RootApproved {
		t.Fatal("a regular file must never be approved as a workspace root")
	}
}

func TestDiscoverStopsOnCancelledContext(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"one"}, []string{"one/package.json"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report := NewSystemInspector().Discover(ctx, root)
	if report.Complete || len(report.Projects) != 0 || len(report.Warnings) == 0 {
		t.Fatalf("a cancelled context must fail closed: %+v", report)
	}
}

type deniedDirectoryReader struct {
	denied string
}

func (r deniedDirectoryReader) ReadDir(name string) ([]fs.DirEntry, error) {
	if filepath.Clean(name) == filepath.Clean(r.denied) {
		return nil, fs.ErrPermission
	}
	return os.ReadDir(name)
}

func TestDiscoverRecordsUnreadableDirectoryWithoutClaimingItIsEmpty(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root,
		[]string{"locked/node_modules"},
		[]string{"package.json", "locked/package.json"},
	)
	locked := filepath.Join(root, "locked")

	report := NewInspectorWithReader(Limits{}, deniedDirectoryReader{denied: locked}).Discover(context.Background(), root)
	if report.Complete {
		t.Fatal("an unreadable directory must mark the snapshot incomplete")
	}
	if len(report.Warnings) == 0 {
		t.Fatal("an unreadable directory must emit a warning")
	}
	if kind, ok := skipKind(report, "locked"); !ok || kind != core.ProjectSkipUnreadable {
		t.Fatalf("expected unreadable skip, got %q (present=%v)", kind, ok)
	}
	if _, discovered := projectByRelativePath(report, "locked"); discovered {
		t.Fatal("an unreadable directory must not produce a project observation")
	}
	if len(report.Projects) != 1 {
		t.Fatalf("only the readable root project may be reported: %+v", report.Projects)
	}
}

func TestIsReparseRejectsSymlinkAndOtherReparseTags(t *testing.T) {
	// Windows reports non-symlink reparse points as irregular, so both bits must be
	// rejected. This runs on every host, including one without symlink privileges.
	for _, mode := range []fs.FileMode{
		fs.ModeSymlink,
		fs.ModeIrregular,
		fs.ModeDir | fs.ModeSymlink,
		fs.ModeDir | fs.ModeIrregular,
	} {
		if !isReparse(mode) {
			t.Fatalf("mode %v must be treated as a reparse point", mode)
		}
	}
	for _, mode := range []fs.FileMode{0, fs.ModeDir, fs.FileMode(0o644)} {
		if isReparse(mode) {
			t.Fatalf("mode %v must not be treated as a reparse point", mode)
		}
	}
}

// plainDirectoryEntry imitates a stale cached entry that still claims a plain
// directory after the resolved path became a reparse point.
type plainDirectoryEntry struct {
	name string
}

func (e plainDirectoryEntry) Name() string               { return e.name }
func (e plainDirectoryEntry) IsDir() bool                { return true }
func (e plainDirectoryEntry) Type() fs.FileMode          { return fs.ModeDir }
func (e plainDirectoryEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrInvalid }

type staleDirectoryReader struct {
	directory string
	entries   []fs.DirEntry
}

func (r staleDirectoryReader) ReadDir(name string) ([]fs.DirEntry, error) {
	if filepath.Clean(name) == filepath.Clean(r.directory) {
		return r.entries, nil
	}
	return os.ReadDir(name)
}

func TestDiscoverRevalidatesResolvedPathBeforeReading(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeFixture(t, root, nil, []string{"package.json"})
	writeFixture(t, outside, []string{"target"}, []string{"Cargo.toml"})
	if err := os.Symlink(outside, filepath.Join(root, "swapped")); err != nil {
		t.Skipf("symlink unsupported on this host: %v", err)
	}

	stale := staleDirectoryReader{
		directory: root,
		entries:   []fs.DirEntry{plainDirectoryEntry{name: "swapped"}},
	}
	report := NewInspectorWithReader(Limits{}, stale).Discover(context.Background(), root)
	if len(report.Projects) != 0 {
		t.Fatalf("a swapped reparse point must not contribute projects: %+v", report.Projects)
	}
	if kind, ok := skipKind(report, "swapped"); !ok || kind != core.ProjectSkipReparsePoint {
		t.Fatalf("expected reparse-point skip from read-time revalidation, got %q (present=%v)", kind, ok)
	}
}

func TestDiscoverRecordsDifferentlyCasedBoundaryNames(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root,
		[]string{".GIT/objects", "Node_Modules/nested"},
		[]string{"Cargo.toml", ".GIT/objects/package.json", "Node_Modules/nested/package.json"},
	)

	report := NewSystemInspector().Discover(context.Background(), root)
	if len(report.Projects) != 1 {
		t.Fatalf("case-different boundary names must not be traversed: %+v", report.Projects)
	}
	if kind, ok := skipKind(report, ".GIT"); !ok || kind != core.ProjectSkipExcludedMetadata {
		t.Fatalf("expected excluded-metadata skip for .GIT, got %q (present=%v)", kind, ok)
	}
	if kind, ok := skipKind(report, "Node_Modules"); !ok || kind != core.ProjectSkipUnclaimedName {
		t.Fatalf("expected unclaimed-name skip for Node_Modules, got %q (present=%v)", kind, ok)
	}
}

func TestDiscoverClaimsDifferentlyCasedArtifactName(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"Target"}, []string{"Cargo.toml"})

	report := NewSystemInspector().Discover(context.Background(), root)
	project, ok := projectByRelativePath(report, ".")
	if !ok {
		t.Fatal("Cargo project was not discovered")
	}
	if len(project.Artifacts) != 1 || project.Artifacts[0].Name != "Target" {
		t.Fatalf("expected the on-disk name to be reported once: %v", artifactNames(project))
	}
	if project.Artifacts[0].Ecosystem != core.EcosystemRust {
		t.Fatalf("unexpected claiming ecosystem: %q", project.Artifacts[0].Ecosystem)
	}
	if _, skipped := skipKind(report, "Target"); skipped {
		t.Fatal("a claimed artifact must not also be recorded as a skip")
	}
}

func TestDiscoverDepthBoundMarksSnapshotIncompleteAndWarnsOnce(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"one/deep", "two/deep"}, []string{"one/deep/package.json", "two/deep/package.json"})

	report := NewInspector(Limits{MaxDepth: 1}).Discover(context.Background(), root)
	if report.Complete {
		t.Fatal("a depth-truncated snapshot must not be presented as complete")
	}
	if !report.Truncated {
		t.Fatal("a depth-truncated snapshot must be marked truncated")
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("expected exactly one depth warning, got %v", report.Warnings)
	}
}

func TestDiscoverBoundsRecordedSkipList(t *testing.T) {
	root := t.TempDir()
	directories := make([]string, 0, maxRecordedSkips+10)
	for index := 0; index < maxRecordedSkips+10; index++ {
		directories = append(directories, filepath.Join("orphans", "p"+strconv.Itoa(index), "dist"))
	}
	writeFixture(t, root, directories, []string{"package.json"})

	report := NewSystemInspector().Discover(context.Background(), root)
	if len(report.Skipped) != maxRecordedSkips {
		t.Fatalf("skip list must be bounded at %d, got %d", maxRecordedSkips, len(report.Skipped))
	}
	if report.Complete {
		t.Fatal("an elided skip list must mark the snapshot incomplete")
	}
}
