package npm

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

type fakeRunner struct {
	executable string
	version    string
	cacheRoot  string
	lookErr    error
	clean      bool
	calls      [][]string
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
	case reflect.DeepEqual(arguments, []string{"config", "get", "cache"}):
		return r.cacheRoot + "\n", nil
	case reflect.DeepEqual(arguments, []string{"cache", "clean", "--force"}):
		if !r.clean {
			return "", errors.New("cleanup not enabled")
		}
		if err := os.RemoveAll(filepath.Join(r.cacheRoot, "_cacache")); err != nil {
			return "", err
		}
		return "npm warn using --force", nil
	default:
		return "", errors.New("unexpected command")
	}
}

func TestInspectNpmMeasuresCacheAndBuildsReviewPlan(t *testing.T) {
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "_cacache", "content"), 29)
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "npm.cmd"), version: "11.8.0", cacheRoot: root}
	inspection, err := core.InspectProvider(context.Background(), NewProvider(runner))
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Detection.Supported || inspection.Detection.Version != "11.8.0" {
		t.Fatalf("unexpected detection: %#v", inspection.Detection)
	}
	if got := inspection.Scan.Items[0].Measured; got.Bytes != 29 || got.Kind != core.MeasurementMeasuredLogical {
		t.Fatalf("unexpected measurement: %#v", got)
	}
	action := inspection.Plan.Actions[0]
	if action.Risk != core.RiskReview || action.RecoveryCost != core.RecoveryDownload || action.Location != root {
		t.Fatalf("unexpected action: %#v", action)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("scan ran unexpected commands: %#v", runner.calls)
	}
}

func TestNpmExecutionRequiresConfirmationAndVerifies(t *testing.T) {
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "_cacache", "entry"), 41)
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "npm.cmd"), version: "10.9.4", cacheRoot: root, clean: true}
	provider := NewProvider(runner)
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Execute(context.Background(), inspection.Plan, false); err == nil {
		t.Fatal("cleanup without confirmation succeeded")
	}
	execution, err := provider.Execute(context.Background(), inspection.Plan, true)
	if err != nil {
		t.Fatal(err)
	}
	if !execution.Executed || !execution.Destructive {
		t.Fatalf("unexpected execution: %#v", execution)
	}
	verification, err := provider.Verify(context.Background(), inspection.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if verification.ReclaimedActual.Bytes != 41 || verification.MeasuredAfter.Bytes != 0 {
		t.Fatalf("unexpected verification: %#v", verification)
	}
}

func TestNpmRejectsUnknownMajorMissingBinaryAndChangedPath(t *testing.T) {
	unsupported, err := core.InspectProvider(context.Background(), NewProvider(&fakeRunner{executable: filepath.Join(t.TempDir(), "npm.cmd"), version: "12.0.0", cacheRoot: t.TempDir()}))
	if err != nil {
		t.Fatal(err)
	}
	if !unsupported.Detection.Detected || unsupported.Detection.Supported || unsupported.Plan.ID != "" {
		t.Fatalf("unexpected unsupported inspection: %#v", unsupported)
	}
	missing, err := core.InspectProvider(context.Background(), NewProvider(&fakeRunner{lookErr: exec.ErrNotFound}))
	if err != nil {
		t.Fatal(err)
	}
	if missing.Detection.Detected {
		t.Fatalf("unexpected missing detection: %#v", missing)
	}

	firstRoot := t.TempDir()
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "npm.cmd"), version: "11.8.0", cacheRoot: firstRoot, clean: true}
	provider := NewProvider(runner)
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	runner.cacheRoot = t.TempDir()
	if _, err := provider.Execute(context.Background(), inspection.Plan, true); err == nil || err.Error() != "npm cache path changed after review; inspect again before cleanup" {
		t.Fatalf("unexpected path error: %v", err)
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
