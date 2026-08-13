package providers_test

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
	cypressprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/cypress"
	nugetprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/nuget"
	yarnprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/yarn"
)

type sliceRunner struct {
	executable string
	kind       string
	version    string
	root       string
	clean      bool
	calls      [][]string
}

func (r *sliceRunner) LookPath(string) (string, error) { return r.executable, nil }
func (r *sliceRunner) Run(_ context.Context, executable string, arguments ...string) (string, error) {
	r.calls = append(r.calls, append([]string{executable}, arguments...))
	if reflect.DeepEqual(arguments, []string{"--version"}) {
		return r.version, nil
	}
	switch r.kind {
	case "yarn":
		if reflect.DeepEqual(arguments, []string{"cache", "dir"}) {
			return r.root + "\n", nil
		}
		if reflect.DeepEqual(arguments, []string{"cache", "clean"}) && r.clean {
			return "", os.RemoveAll(r.root)
		}
	case "nuget":
		if reflect.DeepEqual(arguments, []string{"nuget", "locals", "http-cache", "--list", "--force-english-output"}) {
			return "http-cache: " + r.root + "\n", nil
		}
		if reflect.DeepEqual(arguments, []string{"nuget", "locals", "http-cache", "--clear", "--force-english-output"}) && r.clean {
			return "", os.RemoveAll(r.root)
		}
	case "cypress":
		if reflect.DeepEqual(arguments, []string{"cache", "path"}) {
			return r.root + "\n", nil
		}
		if reflect.DeepEqual(arguments, []string{"cache", "prune"}) && r.clean {
			return "", os.RemoveAll(filepath.Join(r.root, "old"))
		}
	}
	return "", errors.New("unexpected command")
}

func TestGlobalProviderSlicesInspectConfirmAndVerify(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		version     string
		newProvider func(*sliceRunner) core.Provider
		estimated   core.MeasurementKind
		risk        core.RiskLevel
		reclaimed   uint64
		after       uint64
	}{
		{"Yarn Classic", "yarn", "1.22.22", func(r *sliceRunner) core.Provider { return yarnprovider.NewProvider(r) }, core.MeasurementEstimatedLogical, core.RiskReview, 31, 0},
		{"NuGet HTTP cache", "nuget", "8.0.408", func(r *sliceRunner) core.Provider { return nugetprovider.NewProvider(r) }, core.MeasurementEstimatedLogical, core.RiskSafe, 31, 0},
		{"Cypress binary cache", "cypress", "Cypress package version: 14.3.0", func(r *sliceRunner) core.Provider { return cypressprovider.NewProvider(r) }, core.MeasurementUnavailable, core.RiskSafe, 23, 17},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.kind == "cypress" {
				writeSizedFile(t, filepath.Join(root, "old", "binary"), 23)
				writeSizedFile(t, filepath.Join(root, "current", "binary"), 17)
			} else {
				writeSizedFile(t, filepath.Join(root, "cache", "entry"), 31)
			}
			runner := &sliceRunner{executable: filepath.Join(t.TempDir(), test.kind+".exe"), kind: test.kind, version: test.version, root: root, clean: true}
			provider := test.newProvider(runner)
			inspection, err := core.InspectProvider(context.Background(), provider)
			if err != nil {
				t.Fatal(err)
			}
			if !inspection.Detection.Detected || !inspection.Detection.Supported || !inspection.ExecutionEnabled {
				t.Fatalf("unexpected detection: %#v", inspection.Detection)
			}
			action := inspection.Plan.Actions[0]
			if action.Risk != test.risk || action.Estimated.Kind != test.estimated {
				t.Fatalf("unexpected plan: %#v", action)
			}
			if _, err := provider.Execute(context.Background(), inspection.Plan, false); err == nil {
				t.Fatal("cleanup without confirmation succeeded")
			}
			if _, err := provider.Execute(context.Background(), inspection.Plan, true); err != nil {
				t.Fatal(err)
			}
			verification, err := provider.Verify(context.Background(), inspection.Plan)
			if err != nil {
				t.Fatal(err)
			}
			if verification.ReclaimedActual.Bytes != test.reclaimed || verification.MeasuredAfter.Bytes != test.after {
				t.Fatalf("unexpected verification: %#v", verification)
			}
		})
	}
}

func TestGlobalProviderSlicesRejectUnsupportedVersionsAndChangedPaths(t *testing.T) {
	tests := []struct {
		name, kind, version string
		newProvider         func(*sliceRunner) core.Provider
	}{
		{"Yarn Modern", "yarn", "4.6.0", func(r *sliceRunner) core.Provider { return yarnprovider.NewProvider(r) }},
		{"Old SDK", "nuget", "5.0.408", func(r *sliceRunner) core.Provider { return nugetprovider.NewProvider(r) }},
		{"Unsupported Cypress", "cypress", "Cypress package version: 12.9.0", func(r *sliceRunner) core.Provider { return cypressprovider.NewProvider(r) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &sliceRunner{executable: filepath.Join(t.TempDir(), test.kind+".exe"), kind: test.kind, version: test.version, root: t.TempDir()}
			inspection, err := core.InspectProvider(context.Background(), test.newProvider(runner))
			if err != nil {
				t.Fatal(err)
			}
			if !inspection.Detection.Detected || inspection.Detection.Supported || inspection.Plan.ID != "" {
				t.Fatalf("unexpected unsupported inspection: %#v", inspection)
			}
		})
	}
	root := t.TempDir()
	writeSizedFile(t, filepath.Join(root, "cache", "entry"), 5)
	runner := &sliceRunner{executable: filepath.Join(t.TempDir(), "yarn.exe"), kind: "yarn", version: "1.22.22", root: root, clean: true}
	provider := yarnprovider.NewProvider(runner)
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	runner.root = t.TempDir()
	if _, err := provider.Execute(context.Background(), inspection.Plan, true); err == nil || err.Error() != "cache path changed after review; inspect again before cleanup" {
		t.Fatalf("unexpected changed-path error: %v", err)
	}
}

func TestGlobalProviderSlicesTreatMissingExecutablesAsDetectionOnly(t *testing.T) {
	for _, newProvider := range []func(common.CommandRunner) core.Provider{
		func(r common.CommandRunner) core.Provider { return yarnprovider.NewProvider(r) },
		func(r common.CommandRunner) core.Provider { return nugetprovider.NewProvider(r) },
		func(r common.CommandRunner) core.Provider { return cypressprovider.NewProvider(r) },
	} {
		runner := &missingRunner{}
		inspection, err := core.InspectProvider(context.Background(), newProvider(runner))
		if err != nil {
			t.Fatal(err)
		}
		if inspection.Detection.Detected || inspection.Plan.ID != "" {
			t.Fatalf("unexpected missing-tool inspection: %#v", inspection)
		}
	}
}

type missingRunner struct{}

func (*missingRunner) LookPath(string) (string, error) { return "", exec.ErrNotFound }
func (*missingRunner) Run(context.Context, string, ...string) (string, error) {
	return "", errors.New("should not run")
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
