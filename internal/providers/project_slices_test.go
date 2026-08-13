package providers_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	cargoprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/cargo"
	gradleprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/gradle"
	mavenprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/maven"
	playwrightprovider "github.com/the-khiem7/PenguinSpaceStudio/internal/providers/playwright"
)

type projectRunner struct {
	executable, version, kind, root string
	clean                           bool
	calls                           [][]string
}

func (r *projectRunner) LookPath(string) (string, error) { return r.executable, nil }
func (r *projectRunner) Run(_ context.Context, executable string, arguments ...string) (string, error) {
	r.calls = append(r.calls, append([]string{executable}, arguments...))
	if reflect.DeepEqual(arguments, []string{"--version"}) {
		return r.version, nil
	}
	switch r.kind {
	case "cargo":
		if reflect.DeepEqual(arguments, []string{"clean", "--manifest-path", filepath.Join(r.root, "Cargo.toml")}) && r.clean {
			return "", os.RemoveAll(filepath.Join(r.root, "target"))
		}
	case "maven":
		if reflect.DeepEqual(arguments, []string{"-f", filepath.Join(r.root, "pom.xml"), "clean"}) && r.clean {
			return "", os.RemoveAll(filepath.Join(r.root, "target"))
		}
	case "gradle":
		if reflect.DeepEqual(arguments, []string{"--no-daemon", "-p", r.root, ":clean"}) && r.clean {
			return "", os.RemoveAll(filepath.Join(r.root, "build"))
		}
	}
	return "", errors.New("unexpected command")
}

func TestCargoGradleAndMavenWorkspaceLifecycle(t *testing.T) {
	tests := []struct {
		name, kind, manifest, version string
		newProvider                   func(*projectRunner) core.WorkspaceScopedProvider
	}{
		{"Cargo", "cargo", "Cargo.toml", "cargo 1.78.0", func(r *projectRunner) core.WorkspaceScopedProvider { return cargoprovider.NewProvider(r) }},
		{"Gradle", "gradle", "build.gradle", "\nGradle 8.12\n", func(r *projectRunner) core.WorkspaceScopedProvider { return gradleprovider.NewProvider(r) }},
		{"Maven", "maven", "pom.xml", "Apache Maven 3.9.9", func(r *projectRunner) core.WorkspaceScopedProvider { return mavenprovider.NewProvider(r) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeProjectFile(t, filepath.Join(root, test.manifest), "fixture")
			if test.kind == "gradle" {
				writeProjectFile(t, filepath.Join(root, "gradlew"), "fixture")
			}
			writeProjectFile(t, filepath.Join(root, "target", "old"), string(make([]byte, 31)))
			if test.kind == "gradle" {
				if err := os.RemoveAll(filepath.Join(root, "target")); err != nil {
					t.Fatal(err)
				}
				writeProjectFile(t, filepath.Join(root, "build", "old"), string(make([]byte, 31)))
			}
			runner := &projectRunner{executable: filepath.Join(t.TempDir(), test.kind), version: test.version, kind: test.kind, root: root, clean: true}
			provider := test.newProvider(runner)
			if err := provider.SetWorkspaceRoot(root); err != nil {
				t.Fatal(err)
			}
			inspection, err := core.InspectProvider(context.Background(), provider)
			if err != nil {
				t.Fatal(err)
			}
			if !inspection.Detection.Supported || inspection.Scan.Items[0].Measured.Bytes != 31 || inspection.Plan.Actions[0].Risk != core.RiskReview {
				t.Fatalf("unexpected inspection: %#v", inspection)
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
			if verification.ReclaimedActual.Bytes != 31 || verification.MeasuredAfter.Bytes != 0 {
				t.Fatalf("unexpected verification: %#v", verification)
			}
		})
	}
}

func TestProjectProvidersRequireWorkspaceAndRejectTargetDrift(t *testing.T) {
	runner := &projectRunner{executable: filepath.Join(t.TempDir(), "cargo"), version: "cargo 1.78.0", kind: "cargo", clean: true}
	provider := cargoprovider.NewProvider(runner)
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Detection.Detected || inspection.Plan.ID != "" {
		t.Fatalf("unexpected no-workspace inspection: %#v", inspection)
	}
	root := t.TempDir()
	writeProjectFile(t, filepath.Join(root, "Cargo.toml"), "fixture")
	writeProjectFile(t, filepath.Join(root, "target", "old"), "old")
	runner.root = root
	if err := provider.SetWorkspaceRoot(root); err != nil {
		t.Fatal(err)
	}
	inspection, err = core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	changedRoot := t.TempDir()
	writeProjectFile(t, filepath.Join(changedRoot, "Cargo.toml"), "fixture")
	runner.root = changedRoot
	if err := provider.SetWorkspaceRoot(changedRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Execute(context.Background(), inspection.Plan, true); err == nil {
		t.Fatal("cleanup succeeded after workspace root changed")
	}
}

func writeProjectFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

type playwrightRunner struct {
	version, root string
	calls         [][]string
}

func (r *playwrightRunner) LookPath(string) (string, error) {
	return "", errors.New("local CLI must not call LookPath")
}
func (r *playwrightRunner) Run(_ context.Context, executable string, arguments ...string) (string, error) {
	r.calls = append(r.calls, append([]string{executable}, arguments...))
	if reflect.DeepEqual(arguments, []string{"--version"}) {
		return r.version, nil
	}
	return "", errors.New("unexpected command")
}
func (r *playwrightRunner) RunWithEnv(_ context.Context, executable string, environment []string, arguments ...string) (string, error) {
	r.calls = append(r.calls, append([]string{executable}, arguments...))
	if !reflect.DeepEqual(environment, []string{"PLAYWRIGHT_BROWSERS_PATH=0"}) || !reflect.DeepEqual(arguments, []string{"uninstall"}) {
		return "", errors.New("unexpected hermetic uninstall command")
	}
	return "", os.RemoveAll(filepath.Join(r.root, "node_modules", "playwright-core", ".local-browsers"))
}

func TestPlaywrightHermeticWorkspaceLifecycle(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, filepath.Join(root, "package.json"), "{}")
	writeProjectFile(t, filepath.Join(root, "node_modules", ".bin", "playwright"), "fixture")
	writeProjectFile(t, filepath.Join(root, "node_modules", "playwright-core", ".local-browsers", "chromium"), string(make([]byte, 29)))
	runner := &playwrightRunner{version: "Version 1.52.0", root: root}
	provider := playwrightprovider.NewProvider(runner)
	if err := provider.SetWorkspaceRoot(root); err != nil {
		t.Fatal(err)
	}
	inspection, err := core.InspectProvider(context.Background(), provider)
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Detection.Supported || inspection.Scan.Items[0].Measured.Bytes != 29 || inspection.Plan.Actions[0].Estimated.Kind != core.MeasurementEstimatedLogical {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}
	if _, err := provider.Execute(context.Background(), inspection.Plan, false); err == nil {
		t.Fatal("hermetic uninstall without confirmation succeeded")
	}
	if _, err := provider.Execute(context.Background(), inspection.Plan, true); err != nil {
		t.Fatal(err)
	}
	verification, err := provider.Verify(context.Background(), inspection.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if verification.ReclaimedActual.Bytes != 29 || verification.MeasuredAfter.Bytes != 0 {
		t.Fatalf("unexpected verification: %#v", verification)
	}
}
