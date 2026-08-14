package projectinventory

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

func writeSizedFile(t *testing.T, root, relative string, size int) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
}

func nodeProjectFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, nil, []string{"package.json"})
	writeSizedFile(t, root, "node_modules/a.bin", 1000)
	writeSizedFile(t, root, "node_modules/pkg/b.bin", 2000)
	writeSizedFile(t, root, "node_modules/pkg/nested/c.bin", 4000)
	writeSizedFile(t, root, "dist/out.bin", 8000)
	// Source outside any claimed artifact must never be counted.
	writeSizedFile(t, root, "src/index.ts", 500000)
	return root
}

func measurementFor(t *testing.T, measurement core.ProjectMeasurement, name string) core.ProjectArtifactMeasurement {
	t.Helper()
	for _, artifact := range measurement.Artifacts {
		if artifact.Name == name {
			return artifact
		}
	}
	t.Fatalf("artifact %q was not measured: %+v", name, measurement.Artifacts)
	return core.ProjectArtifactMeasurement{}
}

func TestMeasureProjectCountsExactLogicalBytesOfClaimedArtifactsOnly(t *testing.T) {
	root := nodeProjectFixture(t)

	measurement, err := NewSystemInspector().MeasureProject(context.Background(), root, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !measurement.Complete || measurement.Truncated {
		t.Fatalf("expected a complete measurement: %+v", measurement)
	}
	if measurement.Total.Kind != core.MeasurementMeasuredLogical || measurement.Total.Bytes != 15000 {
		t.Fatalf("unexpected project total: %+v", measurement.Total)
	}
	if measurement.Reclaimable.Kind != core.MeasurementUnavailable {
		t.Fatalf("reclaimable bytes must stay unavailable: %+v", measurement.Reclaimable)
	}

	modules := measurementFor(t, measurement, "node_modules")
	if modules.Measured.Bytes != 7000 || modules.Files != 3 || modules.Directories != 3 {
		t.Fatalf("unexpected node_modules measurement: %+v", modules)
	}
	if modules.Reclaimable.Kind != core.MeasurementUnavailable || !modules.Complete {
		t.Fatalf("unexpected node_modules state: %+v", modules)
	}
	dist := measurementFor(t, measurement, "dist")
	if dist.Measured.Bytes != 8000 || dist.Files != 1 {
		t.Fatalf("unexpected dist measurement: %+v", dist)
	}
}

func TestMeasureProjectAppliesExclusionsAndReportsThem(t *testing.T) {
	root := nodeProjectFixture(t)

	measurement, err := NewSystemInspector().MeasureProject(context.Background(), root, root, []string{"node_modules/pkg/nested"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	modules := measurementFor(t, measurement, "node_modules")
	if modules.Measured.Bytes != 3000 {
		t.Fatalf("exclusion must remove exactly the excluded bytes: %+v", modules.Measured)
	}
	if modules.Complete {
		t.Fatal("an excluded path must clear artifact completeness")
	}
	if measurement.Complete {
		t.Fatal("an excluded path must clear project completeness")
	}
	if measurement.Total.Bytes != 11000 {
		t.Fatalf("unexpected scoped total: %+v", measurement.Total)
	}
	if len(measurement.Exclusions) != 1 || !measurement.Exclusions[0].Matched {
		t.Fatalf("the applied exclusion must be reported as matched: %+v", measurement.Exclusions)
	}
	if measurement.Exclusions[0].RelativePath != "node_modules/pkg/nested" {
		t.Fatalf("unexpected reported rule: %+v", measurement.Exclusions[0])
	}
	var excluded int
	for _, skip := range modules.Skipped {
		if skip.Kind == core.ProjectSkipExcludedByRule {
			excluded++
			if skip.RelativePath != "node_modules/pkg/nested" {
				t.Fatalf("unexpected excluded path: %+v", skip)
			}
		}
	}
	if excluded != 1 {
		t.Fatalf("expected exactly one excluded record, got %d (%+v)", excluded, modules.Skipped)
	}
	if !strings.Contains(measurement.Message, "excluded by rule") {
		t.Fatalf("the message must disclose the exclusion: %q", measurement.Message)
	}
}

func TestMeasureProjectExcludesAnEntireArtifact(t *testing.T) {
	root := nodeProjectFixture(t)

	measurement, err := NewSystemInspector().MeasureProject(context.Background(), root, root, []string{"dist"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	dist := measurementFor(t, measurement, "dist")
	if dist.Measured.Kind != core.MeasurementUnavailable || dist.Measured.Bytes != 0 {
		t.Fatalf("an excluded artifact has unknown bytes, not zero measured bytes: %+v", dist.Measured)
	}
	if dist.Files != 0 || dist.Complete {
		t.Fatalf("an excluded artifact must count nothing and stay incomplete: %+v", dist)
	}
	if measurement.Total.Bytes != 7000 {
		t.Fatalf("unexpected scoped total: %+v", measurement.Total)
	}
	var disclosed bool
	for _, warning := range measurement.Warnings {
		if strings.Contains(warning, "unknown bytes") {
			disclosed = true
		}
	}
	if !disclosed {
		t.Fatalf("an artifact left out of the total must be disclosed: %+v", measurement.Warnings)
	}
}

func TestMeasureProjectMarksOverlappingExclusionAsMatched(t *testing.T) {
	root := nodeProjectFixture(t)

	measurement, err := NewSystemInspector().MeasureProject(context.Background(), root, root, []string{"node_modules", "node_modules/pkg/nested"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(measurement.Exclusions) != 2 {
		t.Fatalf("both rules must be reported: %+v", measurement.Exclusions)
	}
	for _, rule := range measurement.Exclusions {
		if !rule.Matched {
			t.Fatalf("a rule subsumed by a broader applied rule must not be reported as unmatched: %+v", rule)
		}
	}
}

func TestMeasureProjectRejectsExclusionThroughReparsePoint(t *testing.T) {
	root := nodeProjectFixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "dist", "linked")); err != nil {
		t.Skipf("symlink unsupported on this host: %v", err)
	}

	if _, err := NewSystemInspector().MeasureProject(context.Background(), root, root, []string{"dist/linked"}, nil); err == nil {
		t.Fatal("an exclusion naming a reparse point was accepted")
	}
	if _, err := NewSystemInspector().MeasureProject(context.Background(), root, root, []string{"dist/linked/inside"}, nil); err == nil {
		t.Fatal("an exclusion passing through a reparse point was accepted")
	}
}

type symlinkEntryReader struct {
	directory string
	name      string
}

// ReadDir replaces one real directory entry with a symbolic-link entry, imitating a
// link created after the exclusion rule was validated.
func (r symlinkEntryReader) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil || filepath.Clean(name) != filepath.Clean(r.directory) {
		return entries, err
	}
	kept := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() != r.name {
			kept = append(kept, entry)
		}
	}
	return append(kept, symlinkEntry{name: r.name}), nil
}

type symlinkEntry struct {
	name string
}

func (e symlinkEntry) Name() string               { return e.name }
func (e symlinkEntry) IsDir() bool                { return false }
func (e symlinkEntry) Type() fs.FileMode          { return fs.ModeSymlink | fs.ModeDir }
func (e symlinkEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrInvalid }

func TestMeasureProjectPrefersSafetySkipOverExclusion(t *testing.T) {
	root := nodeProjectFixture(t)
	writeSizedFile(t, root, "dist/later/inside.bin", 3000)
	reader := symlinkEntryReader{directory: filepath.Join(root, "dist"), name: "later"}

	measurement, err := NewInspectorWithReader(Limits{}, reader).MeasureProject(context.Background(), root, root, []string{"dist/later"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	dist := measurementFor(t, measurement, "dist")
	if dist.Measured.Bytes != 8000 {
		t.Fatalf("neither the link nor its target may contribute bytes: %+v", dist.Measured)
	}
	for _, skip := range dist.Skipped {
		if skip.RelativePath != "dist/later" {
			continue
		}
		if skip.Kind != core.ProjectSkipReparsePoint {
			t.Fatalf("a safety rule must classify the path even when an exclusion also covers it: %+v", skip)
		}
		return
	}
	t.Fatalf("expected the link to be recorded: %+v", dist.Skipped)
}

type hugeFileReader struct {
	directory string
	files     int
}

func (r hugeFileReader) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil || filepath.Clean(name) != filepath.Clean(r.directory) {
		return entries, err
	}
	for index := 0; index < r.files; index++ {
		entries = append(entries, hugeFileEntry{name: "huge" + strconv.Itoa(index) + ".bin"})
	}
	return entries, nil
}

type hugeFileEntry struct {
	name string
}

func (e hugeFileEntry) Name() string               { return e.name }
func (e hugeFileEntry) IsDir() bool                { return false }
func (e hugeFileEntry) Type() fs.FileMode          { return 0 }
func (e hugeFileEntry) Info() (fs.FileInfo, error) { return hugeFileInfo{name: e.name}, nil }

type hugeFileInfo struct {
	name string
}

func (i hugeFileInfo) Name() string       { return i.name }
func (i hugeFileInfo) Size() int64        { return math.MaxInt64 }
func (i hugeFileInfo) Mode() fs.FileMode  { return 0o644 }
func (i hugeFileInfo) ModTime() time.Time { return time.Time{} }
func (i hugeFileInfo) IsDir() bool        { return false }
func (i hugeFileInfo) Sys() any           { return nil }

func TestMeasureProjectStopsOnByteOverflow(t *testing.T) {
	root := nodeProjectFixture(t)
	reader := hugeFileReader{directory: filepath.Join(root, "dist"), files: 3}

	measurement, err := NewInspectorWithReader(Limits{}, reader).MeasureProject(context.Background(), root, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dist := measurementFor(t, measurement, "dist")
	if dist.Complete || !dist.Truncated {
		t.Fatalf("an overflowing artifact must stop and report a partial count: %+v", dist)
	}
	var counted uint64 = math.MaxUint64 - 1 // two MaxInt64 files fit exactly; the third overflows.
	if dist.Measured.Bytes != counted {
		t.Fatalf("the overflowing addition must not wrap the counted bytes: %+v", dist.Measured)
	}
	if measurement.Complete {
		t.Fatal("an overflowing artifact must clear project completeness")
	}
	if measurement.Total.Kind != core.MeasurementUnavailable {
		t.Fatalf("an overflowing project total must become unavailable rather than wrap: %+v", measurement.Total)
	}
}

func TestMeasureProjectCountsSparseFileLogicalSize(t *testing.T) {
	root := nodeProjectFixture(t)
	sparse := filepath.Join(root, "dist", "sparse.bin")
	if err := os.WriteFile(sparse, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(sparse, 1<<20); err != nil {
		t.Skipf("sparse truncate unsupported: %v", err)
	}

	measurement, err := NewSystemInspector().MeasureProject(context.Background(), root, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dist := measurementFor(t, measurement, "dist")
	if dist.Measured.Bytes != 8000+(1<<20) {
		t.Fatalf("a sparse file must contribute its logical size, not its allocation: %+v", dist.Measured)
	}
	if !strings.Contains(measurement.Boundary, "sparse files") {
		t.Fatalf("the measurement boundary must disclose the sparse-file caveat: %q", measurement.Boundary)
	}
}

func TestMeasureProjectDistinguishesIncompleteDiscoveryFromUnknownProject(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, []string{"one", "two"}, []string{"one/package.json", "two/package.json"})

	bounded := NewInspector(Limits{MaxDirectories: 2})
	_, err := bounded.MeasureProject(context.Background(), root, filepath.Join(root, "two"), nil, nil)
	if err == nil {
		t.Fatal("an incomplete discovery must not silently measure")
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("an incomplete discovery must not be reported as a missing project: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewSystemInspector().MeasureProject(ctx, root, filepath.Join(root, "one"), nil, nil); err == nil || !strings.Contains(err.Error(), "did not finish") {
		t.Fatalf("a cancelled discovery must be reported as such: %v", err)
	}
}

func TestMeasureProjectReportsUnmatchedExclusion(t *testing.T) {
	root := nodeProjectFixture(t)

	measurement, err := NewSystemInspector().MeasureProject(context.Background(), root, root, []string{"src"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Total.Bytes != 15000 {
		t.Fatalf("an unmatched exclusion must not change the total: %+v", measurement.Total)
	}
	if len(measurement.Exclusions) != 1 || measurement.Exclusions[0].Matched {
		t.Fatalf("expected one unmatched exclusion: %+v", measurement.Exclusions)
	}
	if len(measurement.Warnings) == 0 {
		t.Fatal("an unmatched exclusion must warn that it removed nothing")
	}
}

func TestMeasureProjectRejectsUnsafeExclusions(t *testing.T) {
	root := nodeProjectFixture(t)
	inspector := NewSystemInspector()

	for name, exclusion := range map[string]string{
		"escaping":        filepath.Join(filepath.Dir(root), "outside"),
		"parent-relative": filepath.Join("..", "outside"),
		"root-itself":     root,
		"pattern":         "node_modules/*",
	} {
		if _, err := inspector.MeasureProject(context.Background(), root, root, []string{exclusion}, nil); err == nil {
			t.Fatalf("%s exclusion was accepted", name)
		}
	}
}

func TestMeasureProjectRejectsUnknownProjectPath(t *testing.T) {
	root := nodeProjectFixture(t)
	inspector := NewSystemInspector()

	if _, err := inspector.MeasureProject(context.Background(), root, filepath.Join(root, "src"), nil, nil); err == nil {
		t.Fatal("a directory without markers was accepted as a project")
	}
	if _, err := inspector.MeasureProject(context.Background(), root, filepath.Join(root, "node_modules"), nil, nil); err == nil {
		t.Fatal("a claimed artifact was accepted as a project")
	}
	if _, err := inspector.MeasureProject(context.Background(), root, "", nil, nil); err == nil {
		t.Fatal("an empty project path was accepted")
	}
	if _, err := inspector.MeasureProject(context.Background(), filepath.Join(root, "absent"), root, nil, nil); err == nil {
		t.Fatal("an unapproved root was accepted")
	}
}

func TestMeasureProjectDoesNotFollowReparsePoints(t *testing.T) {
	root := nodeProjectFixture(t)
	outside := t.TempDir()
	writeSizedFile(t, outside, "huge.bin", 64000)
	if err := os.Symlink(outside, filepath.Join(root, "dist", "linked")); err != nil {
		t.Skipf("symlink unsupported on this host: %v", err)
	}

	measurement, err := NewSystemInspector().MeasureProject(context.Background(), root, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dist := measurementFor(t, measurement, "dist")
	if dist.Measured.Bytes != 8000 {
		t.Fatalf("a reparse point must not contribute bytes: %+v", dist.Measured)
	}
	if dist.Complete || measurement.Complete {
		t.Fatal("a recorded reparse point must clear completeness")
	}
	var found bool
	for _, skip := range dist.Skipped {
		if skip.Kind == core.ProjectSkipReparsePoint && skip.RelativePath == "dist/linked" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a reparse-point skip: %+v", dist.Skipped)
	}
}

type irregularEntryReader struct {
	directory string
	name      string
}

func (r irregularEntryReader) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil || filepath.Clean(name) != filepath.Clean(r.directory) {
		return entries, err
	}
	return append(entries, irregularEntry{name: r.name}), nil
}

type irregularEntry struct {
	name string
}

func (e irregularEntry) Name() string               { return e.name }
func (e irregularEntry) IsDir() bool                { return false }
func (e irregularEntry) Type() fs.FileMode          { return fs.ModeDevice }
func (e irregularEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrInvalid }

func TestMeasureProjectRecordsNonRegularEntry(t *testing.T) {
	root := nodeProjectFixture(t)
	reader := irregularEntryReader{directory: filepath.Join(root, "dist"), name: "device"}

	measurement, err := NewInspectorWithReader(Limits{}, reader).MeasureProject(context.Background(), root, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dist := measurementFor(t, measurement, "dist")
	if dist.Measured.Bytes != 8000 || dist.Files != 1 {
		t.Fatalf("a non-regular entry must not contribute bytes: %+v", dist)
	}
	if dist.Complete {
		t.Fatal("a non-regular entry must clear completeness")
	}
	var found bool
	for _, skip := range dist.Skipped {
		if skip.Kind == core.ProjectSkipNonRegular && skip.RelativePath == "dist/device" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a non-regular skip: %+v", dist.Skipped)
	}
}

func TestMeasureProjectRecordsUnreadableDirectory(t *testing.T) {
	root := nodeProjectFixture(t)
	denied := filepath.Join(root, "node_modules", "pkg")

	measurement, err := NewInspectorWithReader(Limits{}, deniedDirectoryReader{denied: denied}).MeasureProject(context.Background(), root, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	modules := measurementFor(t, measurement, "node_modules")
	if modules.Measured.Bytes != 1000 {
		t.Fatalf("an unreadable subtree must not be counted: %+v", modules.Measured)
	}
	if modules.Complete || measurement.Complete {
		t.Fatal("an unreadable directory must clear completeness")
	}
	var found bool
	for _, skip := range modules.Skipped {
		if skip.Kind == core.ProjectSkipUnreadable && skip.RelativePath == "node_modules/pkg" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an unreadable skip: %+v", modules.Skipped)
	}
	if !strings.Contains(measurement.Message, "skipped for safety") {
		t.Fatalf("the message must disclose the safety skip: %q", measurement.Message)
	}
}

func TestMeasureProjectEnforcesEntryBudget(t *testing.T) {
	root := nodeProjectFixture(t)

	measurement, err := NewInspector(Limits{MaxMeasuredEntries: 2}).MeasureProject(context.Background(), root, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !measurement.Truncated || measurement.Complete {
		t.Fatalf("the entry budget must produce an explicit partial count: %+v", measurement)
	}
	if measurement.Total.Bytes >= 15000 {
		t.Fatalf("a truncated measurement cannot report the full total: %+v", measurement.Total)
	}
	if len(measurement.Warnings) == 0 || !strings.Contains(measurement.Message, "partial count") {
		t.Fatalf("truncation must be disclosed: %+v / %q", measurement.Warnings, measurement.Message)
	}
}

func TestMeasureProjectStopsOnCancelledContext(t *testing.T) {
	root := nodeProjectFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewSystemInspector().MeasureProject(ctx, root, root, nil, nil); err == nil {
		t.Fatal("a cancelled context must not produce a measurement")
	}
}

func TestIsWithinRejectsSharedNamePrefix(t *testing.T) {
	base := filepath.Join(string(filepath.Separator)+"workspace", "build")
	if isWithin(base, filepath.Join(string(filepath.Separator)+"workspace", "build-output", "file")) {
		t.Fatal("a sibling with a shared name prefix must not be treated as excluded")
	}
	if !isWithin(base, filepath.Join(base, "nested", "file")) {
		t.Fatal("a descendant must be treated as excluded")
	}
	if !isWithin(base, base) {
		t.Fatal("the exact path must be treated as excluded")
	}
}

// cancelAfterNReadsReader triggers cancel.Cancel() after the Nth ReadDir call to a
// specific directory, imitating a caller cancelling mid-walk on a real filesystem.
type cancelAfterNReadsReader struct {
	directory string
	after     int
	cancel    *CancelSignal
	reads     int
}

func (r *cancelAfterNReadsReader) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err == nil && filepath.Clean(name) == filepath.Clean(r.directory) {
		r.reads++
		if r.reads == r.after {
			r.cancel.Cancel()
		}
	}
	return entries, err
}

func TestMeasureProjectStopsPromptlyOnCancelBetweenArtifacts(t *testing.T) {
	root := nodeProjectFixture(t)
	cancel := &CancelSignal{}
	reader := &cancelAfterNReadsReader{directory: filepath.Join(root, "node_modules"), after: 1, cancel: cancel}

	measurement, err := NewInspectorWithReader(Limits{}, reader).MeasureProject(context.Background(), root, root, nil, cancel)
	if err != nil {
		t.Fatal(err)
	}
	if !measurement.Cancelled {
		t.Fatalf("expected the measurement to report cancellation: %+v", measurement)
	}
	if measurement.Complete || !measurement.Truncated {
		t.Fatalf("a cancelled measurement must not be complete: %+v", measurement)
	}
	dist := measurementFor(t, measurement, "dist")
	if dist.Measured.Bytes != 8000 {
		t.Fatalf("an artifact fully measured before the cancel must keep its exact bytes: %+v", dist.Measured)
	}
	if !strings.Contains(measurement.Message, "cancelled") {
		t.Fatalf("cancellation must be disclosed in the message: %q", measurement.Message)
	}
}

func TestMeasureProjectStopsPromptlyOnCancelMidDirectory(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, nil, []string{"package.json"})
	for index := 0; index < 5; index++ {
		writeSizedFile(t, root, fmt.Sprintf("dist/f%d.bin", index), 1000)
	}
	cancel := &CancelSignal{}
	cancel.Cancel() // Already cancelled before the walk starts.

	measurement, err := NewSystemInspector().MeasureProject(context.Background(), root, root, nil, cancel)
	if err != nil {
		t.Fatal(err)
	}
	if !measurement.Cancelled {
		t.Fatalf("a pre-cancelled signal must stop before any artifact is measured: %+v", measurement)
	}
	if measurement.Complete {
		t.Fatal("a pre-cancelled measurement must not be complete")
	}
	if len(measurement.Artifacts) != 0 {
		t.Fatalf("a signal cancelled before the walk starts must measure nothing: %+v", measurement.Artifacts)
	}
	if measurement.Total.Kind != core.MeasurementMeasuredLogical || measurement.Total.Bytes != 0 {
		t.Fatalf("an empty artifact list sums to zero measured bytes: %+v", measurement.Total)
	}
}

func TestMeasureProjectNilCancelSignalNeverCancels(t *testing.T) {
	root := nodeProjectFixture(t)

	measurement, err := NewSystemInspector().MeasureProject(context.Background(), root, root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if measurement.Cancelled || !measurement.Complete {
		t.Fatalf("a nil cancel signal must never cancel a measurement: %+v", measurement)
	}
}

func TestMeasureProjectDoesNotReportCancelledOnNaturalTimeout(t *testing.T) {
	root := nodeProjectFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, err := NewSystemInspector().MeasureProject(ctx, root, root, nil, nil)
	if err == nil {
		t.Fatal("a natural context timeout during discovery must surface as an error, not a silent cancelled result")
	}
}

func TestCancelSignalIsSafeForConcurrentUse(t *testing.T) {
	signal := &CancelSignal{}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			_ = signal.cancelledNow()
		}
		close(done)
	}()
	for i := 0; i < 1000; i++ {
		signal.Cancel()
	}
	<-done
}
