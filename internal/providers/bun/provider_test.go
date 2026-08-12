package bun

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

type fakeRunner struct {
	executable string
	version    string
	cacheRoot  string
	lookErr    error
	calls      [][]string
	clean      bool
}

func (r *fakeRunner) LookPath(string) (string, error) {
	if r.lookErr != nil {
		return "", r.lookErr
	}
	return r.executable, nil
}

func (r *fakeRunner) Run(_ context.Context, executable string, arguments ...string) (string, error) {
	r.calls = append(r.calls, append([]string{executable}, arguments...))
	switch {
	case reflect.DeepEqual(arguments, []string{"--version"}):
		return r.version, nil
	case len(arguments) == 4 && arguments[0] == "pm" && arguments[1] == "--cwd" && arguments[3] == "cache":
		manifest, err := os.ReadFile(filepath.Join(arguments[2], "package.json"))
		if err != nil || string(manifest) != "{\"private\":true}\n" {
			return "", errors.New("isolated Bun command context is invalid")
		}
		return r.cacheRoot, nil
	case len(arguments) == 5 && arguments[0] == "pm" && arguments[1] == "--cwd" && arguments[3] == "cache" && arguments[4] == "rm":
		if !r.clean {
			return "", errors.New("cleanup not enabled in fake runner")
		}
		if err := os.RemoveAll(r.cacheRoot); err != nil {
			return "", err
		}
		return "Cleared Bun cache", nil
	default:
		return "", errors.New("unexpected command")
	}
}

func TestInspectProviderMeasuresBunCacheAndBuildsSafePlan(t *testing.T) {
	cacheRoot := t.TempDir()
	writeSizedFile(t, filepath.Join(cacheRoot, "pkg-a", "index.js"), 11)
	writeSizedFile(t, filepath.Join(cacheRoot, "pkg-b", "package.json"), 17)
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "bun.exe"), version: "1.3.14\n", cacheRoot: cacheRoot + "\n"}
	provider := NewProvider(runner)

	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Detection.Detected || !inspection.Detection.Supported || inspection.Detection.Version != "1.3.14" {
		t.Fatalf("unexpected detection: %#v", inspection.Detection)
	}
	if got := inspection.Scan.Items[0].Measured; got.Bytes != 28 || got.Kind != core.MeasurementMeasuredLogical {
		t.Fatalf("unexpected measurement: %#v", got)
	}
	action := inspection.Plan.Actions[0]
	if action.Risk != core.RiskSafe || action.RecoveryCost != core.RecoveryDownload || action.Estimated.Kind != core.MeasurementEstimatedLogical {
		t.Fatalf("unexpected action classification: %#v", action)
	}
	if !inspection.ExecutionEnabled {
		t.Fatal("Bun execution must be enabled after representative Windows verification")
	}
	if len(runner.calls) != 2 || len(runner.calls[1]) != 5 || !reflect.DeepEqual(runner.calls[1][1:3], []string{"pm", "--cwd"}) || runner.calls[1][4] != "cache" {
		t.Fatalf("unexpected command calls: %#v", runner.calls)
	}
	if _, err := os.Stat(runner.calls[1][3]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("isolated Bun command context was not removed: %v", err)
	}
}

func TestUnknownBunMajorIsDetectedWithoutPlan(t *testing.T) {
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "bun.exe"), version: "2.0.0", cacheRoot: t.TempDir()}
	inspection, err := core.InspectProvider(context.Background(), NewProvider(runner))
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Detection.Detected || inspection.Detection.Supported {
		t.Fatalf("unexpected detection: %#v", inspection.Detection)
	}
	if inspection.Plan.ID != "" || len(runner.calls) != 1 {
		t.Fatalf("unsupported Bun must not be scanned or planned: %#v", inspection)
	}
}

func TestMissingBunProducesDetectionOnly(t *testing.T) {
	runner := &fakeRunner{lookErr: exec.ErrNotFound}
	inspection, err := core.InspectProvider(context.Background(), NewProvider(runner))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Detection.Detected || inspection.Detection.Supported || inspection.Plan.ID != "" {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}
}

func TestBunExecutionRequiresConfirmationAndVerifiesLogicalReclaim(t *testing.T) {
	cacheRoot := t.TempDir()
	writeSizedFile(t, filepath.Join(cacheRoot, "package.tgz"), 31)
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "bun.exe"), version: "1.3.14", cacheRoot: cacheRoot, clean: true}
	provider := NewProvider(runner)
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Execute(context.Background(), inspection.Plan, false); err == nil || err.Error() != "cleanup plan requires confirmation" {
		t.Fatalf("unexpected confirmation error: %v", err)
	}
	execution, err := provider.Execute(context.Background(), inspection.Plan, true)
	if err != nil {
		t.Fatal(err)
	}
	if !execution.Executed || !execution.Destructive {
		t.Fatalf("unexpected execution result: %#v", execution)
	}
	verification, err := provider.Verify(context.Background(), inspection.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if verification.MeasuredAfter.Bytes != 0 || verification.ReclaimedActual.Bytes != 31 || verification.ReclaimedActual.Kind != core.MeasurementMeasuredLogical {
		t.Fatalf("unexpected verification: %#v", verification)
	}
	if len(runner.calls) != 7 || !reflect.DeepEqual(runner.calls[4][4:], []string{"cache", "rm"}) {
		t.Fatalf("unexpected execution calls: %#v", runner.calls)
	}
}

func TestBunExecutionRejectsChangedCacheRoot(t *testing.T) {
	firstRoot := t.TempDir()
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "bun.exe"), version: "1.3.14", cacheRoot: firstRoot, clean: true}
	provider := NewProvider(runner)
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	runner.cacheRoot = t.TempDir()
	if _, err := provider.Execute(context.Background(), inspection.Plan, true); err == nil || err.Error() != "Bun cache path changed after review; inspect again before cleanup" {
		t.Fatalf("unexpected changed-path error: %v", err)
	}
}

func TestCacheRootRejectsBroadOrRelativePaths(t *testing.T) {
	for _, path := range []string{"relative/cache", filepath.VolumeName(t.TempDir()) + string(filepath.Separator)} {
		if _, err := common.ValidateStorageRoot(path, "Bun"); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
}

func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatal(err)
	}
}
