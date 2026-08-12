package pnpm

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

type fakeRunner struct {
	executable string
	version    string
	storeRoot  string
	lookErr    error
	clean      bool
	configured bool
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
	case len(arguments) == 5 && arguments[0] == "--dir" && arguments[2] == "config" && arguments[3] == "get" && arguments[4] == "store-dir":
		if r.configured {
			return r.storeRoot + "\n", nil
		}
		return "undefined\n", nil
	case len(arguments) == 4 && arguments[0] == "--dir" && arguments[2] == "store" && arguments[3] == "path":
		if info, err := os.Stat(arguments[1]); err != nil || !info.IsDir() {
			return "", errors.New("invalid isolated command context")
		}
		return r.storeRoot + "\n", nil
	case len(arguments) == 4 && arguments[0] == "--dir" && arguments[2] == "store" && arguments[3] == "prune":
		if !r.clean {
			return "", errors.New("prune not enabled")
		}
		if err := os.RemoveAll(filepath.Join(r.storeRoot, "unreferenced")); err != nil {
			return "", err
		}
		return "Removed 1 package", nil
	default:
		return "", errors.New("unexpected command")
	}
}

func TestPnpmInspectionUsesUnavailableEstimate(t *testing.T) {
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "store", "entry"), 37)
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "pnpm.cmd"), version: "11.3.0", storeRoot: root, configured: true}
	inspection, err := core.InspectProvider(context.Background(), NewProvider(runner))
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Detection.Supported || inspection.Scan.Items[0].Measured.Bytes != 37 {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}
	action := inspection.Plan.Actions[0]
	if action.Observed.Bytes != 37 || action.Estimated.Kind != core.MeasurementUnavailable || action.Estimated.Bytes != 0 || action.Risk != core.RiskSafe {
		t.Fatalf("unexpected action: %#v", action)
	}
}

func TestPnpmPruneRequiresConfirmationAndVerifiesActualDifference(t *testing.T) {
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "unreferenced", "old"), 23)
	writeSizedFile(t, filepath.Join(root, "referenced", "keep"), 17)
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "pnpm.cmd"), version: "12.0.0", storeRoot: root, clean: true, configured: true}
	provider := NewProvider(runner)
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Execute(context.Background(), inspection.Plan, false); err == nil {
		t.Fatal("prune without confirmation succeeded")
	}
	if _, err := provider.Execute(context.Background(), inspection.Plan, true); err != nil {
		t.Fatal(err)
	}
	verification, err := provider.Verify(context.Background(), inspection.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if verification.ReclaimedActual.Bytes != 23 || verification.MeasuredAfter.Bytes != 17 || verification.ReclaimedActual.Kind != core.MeasurementMeasuredLogical {
		t.Fatalf("unexpected verification: %#v", verification)
	}
}

func TestPnpmRejectsUnknownMajorMissingBinaryAndChangedPath(t *testing.T) {
	unsupported, err := core.InspectProvider(context.Background(), NewProvider(&fakeRunner{executable: filepath.Join(t.TempDir(), "pnpm.cmd"), version: "10.0.0", storeRoot: t.TempDir()}))
	if err != nil {
		t.Fatal(err)
	}
	if !unsupported.Detection.Detected || unsupported.Detection.Supported || unsupported.Plan.ID != "" {
		t.Fatalf("unexpected unsupported result: %#v", unsupported)
	}
	missing, err := core.InspectProvider(context.Background(), NewProvider(&fakeRunner{lookErr: exec.ErrNotFound}))
	if err != nil {
		t.Fatal(err)
	}
	if missing.Detection.Detected {
		t.Fatal("missing pnpm detected")
	}
	first := t.TempDir()
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "pnpm.cmd"), version: "11.3.0", storeRoot: first, clean: true, configured: true}
	provider := NewProvider(runner)
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	runner.storeRoot = t.TempDir()
	if _, err := provider.Execute(context.Background(), inspection.Plan, true); err == nil || err.Error() != "pnpm store path changed after review; inspect again before pruning" {
		t.Fatalf("unexpected changed-path error: %v", err)
	}
}

func TestPnpmWithoutExplicitStoreDefersMultiDiskDiscovery(t *testing.T) {
	inspection, err := core.InspectProvider(context.Background(), NewProvider(&fakeRunner{executable: filepath.Join(t.TempDir(), "pnpm.cmd"), version: "11.3.0", storeRoot: t.TempDir()}))
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Detection.Detected || inspection.Detection.Supported || inspection.Plan.ID != "" {
		t.Fatalf("unexpected deferred inspection: %#v", inspection)
	}
	if !strings.Contains(inspection.Detection.Message, "per disk") {
		t.Fatalf("missing multi-disk explanation: %q", inspection.Detection.Message)
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
