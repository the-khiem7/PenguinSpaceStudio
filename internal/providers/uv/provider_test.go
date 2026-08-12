package uv

import (
	"context"
	"errors"
	"fmt"
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
	cacheRoot  string
	lookErr    error
	prune      bool
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
	case len(arguments) == 4 && arguments[0] == "--directory" && arguments[2] == "cache" && arguments[3] == "dir":
		if info, err := os.Stat(arguments[1]); err != nil || !info.IsDir() {
			return "", errors.New("invalid isolated command context")
		}
		return r.cacheRoot + "\n", nil
	case len(arguments) == 6 && arguments[0] == "--directory" && arguments[2] == "--preview-features" && arguments[3] == "cache-size" && arguments[4] == "cache" && arguments[5] == "size":
		if info, err := os.Stat(arguments[1]); err != nil || !info.IsDir() {
			return "", errors.New("invalid isolated command context")
		}
		return measureRoot(r.cacheRoot), nil
	case len(arguments) == 4 && arguments[0] == "--directory" && arguments[2] == "cache" && arguments[3] == "prune":
		if !r.prune {
			return "", errors.New("prune not enabled")
		}
		if err := os.RemoveAll(filepath.Join(r.cacheRoot, "unused")); err != nil {
			return "", err
		}
		if err := os.RemoveAll(filepath.Join(r.cacheRoot, "environments")); err != nil {
			return "", err
		}
		return "Pruned cache", nil
	default:
		return "", errors.New("unexpected command")
	}
}

func measureRoot(root string) string {
	var total int64
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return err
	})
	return fmt.Sprintf("%d\n", total)
}

func TestUvInspectionUsesUnavailableEstimate(t *testing.T) {
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "archive-v0", "entry"), 41)
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "uv.exe"), version: "uv 0.12.1 (build metadata)", cacheRoot: root}
	inspection, err := core.InspectProvider(context.Background(), NewProvider(runner))
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Detection.Supported || inspection.Scan.Items[0].Measured.Bytes != 41 {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}
	if inspection.Detection.Version != "0.12.1" {
		t.Fatalf("unexpected display version: %q", inspection.Detection.Version)
	}
	action := inspection.Plan.Actions[0]
	if action.Observed.Bytes != 41 || action.Estimated.Kind != core.MeasurementUnavailable || action.Estimated.Bytes != 0 || action.Risk != core.RiskSafe || action.RecoveryCost != core.RecoveryDownload {
		t.Fatalf("unexpected action: %#v", action)
	}
	if !strings.Contains(action.Consequence, "centralized project environments") {
		t.Fatalf("missing environment consequence: %q", action.Consequence)
	}
}

func TestUvPruneRequiresConfirmationAndVerifiesActualDifference(t *testing.T) {
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "unused", "old"), 23)
	writeSizedFile(t, filepath.Join(root, "environments", "project"), 19)
	writeSizedFile(t, filepath.Join(root, "referenced", "keep"), 17)
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "uv.exe"), version: "uv 0.12.1", cacheRoot: root, prune: true}
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
	if verification.ReclaimedActual.Bytes != 42 || verification.MeasuredAfter.Bytes != 17 || verification.ReclaimedActual.Kind != core.MeasurementMeasuredLogical {
		t.Fatalf("unexpected verification: %#v", verification)
	}
}

func TestUvRejectsOtherMinorMissingBinaryAndChangedPath(t *testing.T) {
	unsupported, err := core.InspectProvider(context.Background(), NewProvider(&fakeRunner{executable: filepath.Join(t.TempDir(), "uv.exe"), version: "uv 0.11.9", cacheRoot: t.TempDir()}))
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
		t.Fatal("missing uv detected")
	}
	first := t.TempDir()
	runner := &fakeRunner{executable: filepath.Join(t.TempDir(), "uv.exe"), version: "uv 0.12.1", cacheRoot: first, prune: true}
	provider := NewProvider(runner)
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	runner.cacheRoot = t.TempDir()
	if _, err := provider.Execute(context.Background(), inspection.Plan, true); err == nil || err.Error() != "uv cache path changed after review; inspect again before pruning" {
		t.Fatalf("unexpected changed-path error: %v", err)
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
