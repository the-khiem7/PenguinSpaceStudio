package wslinventory

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
)

type fakeRunner struct {
	lookPathErr error
	outputs     map[string][]byte
	errors      map[string]error
	calls       []string
}

func (r *fakeRunner) LookPath(name string) (string, error) {
	if r.lookPathErr != nil {
		return "", r.lookPathErr
	}
	return name, nil
}

func (r *fakeRunner) RunRaw(_ context.Context, _ string, arguments ...string) ([]byte, error) {
	command := strings.Join(arguments, " ")
	r.calls = append(r.calls, command)
	return r.outputs[command], r.errors[command]
}

type fakeRegistrationSource struct {
	registrations []Registration
	err           error
}

func (s fakeRegistrationSource) List(context.Context) ([]Registration, error) {
	return s.registrations, s.err
}

type fakeAllocationSource struct {
	allocations map[string]FileAllocation
	errs        map[string]error
	calls       []string
}

func (f *fakeAllocationSource) MeasureAllocation(path string) (FileAllocation, error) {
	f.calls = append(f.calls, path)
	if err := f.errs[path]; err != nil {
		return FileAllocation{}, err
	}
	if allocation, ok := f.allocations[path]; ok {
		return allocation, nil
	}
	return FileAllocation{}, os.ErrNotExist
}

func TestInspectReportsAllocatedBytesWithoutInventingLogicalUsage(t *testing.T) {
	ubuntuBase := filepath.Join(t.TempDir(), "ubuntu")
	legacyBase := filepath.Join(t.TempDir(), "legacy")
	ubuntuDisk := filepath.Join(ubuntuBase, "ext4.vhdx")
	runner := &fakeRunner{outputs: map[string][]byte{
		"--list --quiet":           encodeUTF16("Ubuntu\r\nLegacy\r\n", binary.LittleEndian, true),
		"--list --running --quiet": encodeUTF16("Ubuntu\r\n", binary.LittleEndian, true),
		"--list --verbose":         encodeUTF16("  NAME      STATUS           VERSION\r\n* Ubuntu    Wird ausgeführt  2\r\n  Legacy    Beendet          1\r\n", binary.LittleEndian, true),
	}}
	allocations := &fakeAllocationSource{allocations: map[string]FileAllocation{
		ubuntuDisk: {AllocatedBytes: 2_147_483_648, EndOfFileBytes: 8_589_934_592},
	}}
	inspector := NewInspector(runner, fakeRegistrationSource{registrations: []Registration{
		{Name: "Ubuntu", BasePath: ubuntuBase},
		{Name: "Legacy", BasePath: legacyBase},
	}}, allocations)

	report := inspector.Inspect(context.Background())
	if !report.CLIAvailable || !report.Available || len(report.Distributions) != 2 {
		t.Fatalf("unexpected WSL report: %#v", report)
	}
	legacy, ubuntu := report.Distributions[0], report.Distributions[1]
	if ubuntu.Name != "Ubuntu" || ubuntu.State != core.WSLStateRunning || !ubuntu.VersionAvailable || ubuntu.Version != 2 {
		t.Fatalf("unexpected Ubuntu observation: %#v", ubuntu)
	}
	if !ubuntu.VHDX.PathAvailable || ubuntu.VHDX.Path != ubuntuDisk || ubuntu.VHDX.PhysicalSize.Kind != core.MeasurementMeasuredPhysical || ubuntu.VHDX.PhysicalSize.Bytes != 2_147_483_648 {
		t.Fatalf("allocated bytes were not used as physical size: %#v", ubuntu.VHDX)
	}
	if ubuntu.VHDX.PhysicalSize.Bytes == 8_589_934_592 || ubuntu.VHDX.LogicalUsage.Kind != core.MeasurementUnavailable || ubuntu.VHDX.Compactable.Kind != core.MeasurementUnavailable {
		t.Fatalf("EOF, logical usage, or compactable bytes were presented as physical evidence: %#v", ubuntu.VHDX)
	}
	if legacy.Name != "Legacy" || legacy.State != core.WSLStateStopped || legacy.Version != 1 || legacy.VHDX.PathAvailable {
		t.Fatalf("unexpected WSL 1 observation: %#v", legacy)
	}
	if len(allocations.calls) != 1 || allocations.calls[0] != ubuntuDisk {
		t.Fatalf("unexpected allocation inspection: %#v", allocations.calls)
	}
	assertReadOnlyCommands(t, runner.calls)
}

func TestInspectRejectsAllRegistryPathsAfterAnySourceError(t *testing.T) {
	base := t.TempDir()
	runner := standardRunner("Ubuntu")
	allocations := &fakeAllocationSource{}
	report := NewInspector(runner, fakeRegistrationSource{
		registrations: []Registration{{Name: "Ubuntu", BasePath: base}},
		err:           errors.New("another registration could not be read"),
	}, allocations).Inspect(context.Background())

	if len(report.Distributions) != 1 || report.Distributions[0].VHDX.PathAvailable || len(allocations.calls) != 0 {
		t.Fatalf("partial registry metadata authorized a path: %#v; calls=%#v", report, allocations.calls)
	}
	if !warningsContain(report.Warnings, "every registry-derived VHDX path was disabled") {
		t.Fatalf("missing fail-closed registry warning: %#v", report.Warnings)
	}
}

func TestInspectPermanentlyRejectsThreeDuplicateRegistrations(t *testing.T) {
	runner := standardRunner("Ubuntu")
	allocations := &fakeAllocationSource{}
	report := NewInspector(runner, fakeRegistrationSource{registrations: []Registration{
		{Name: "Ubuntu", BasePath: filepath.Join(t.TempDir(), "first")},
		{Name: "Ubuntu", BasePath: filepath.Join(t.TempDir(), "second")},
		{Name: "Ubuntu", BasePath: filepath.Join(t.TempDir(), "third")},
	}}, allocations).Inspect(context.Background())

	if report.Distributions[0].VHDX.PathAvailable || len(allocations.calls) != 0 {
		t.Fatalf("third duplicate restored an ambiguous registration: %#v", report)
	}
	if !warningsContain(report.Warnings, "permanently disabled") {
		t.Fatalf("missing ambiguity warning: %#v", report.Warnings)
	}
}

func TestInspectRequiresExactCaseRegistrationIdentity(t *testing.T) {
	runner := standardRunner("Ubuntu")
	allocations := &fakeAllocationSource{}
	report := NewInspector(runner, fakeRegistrationSource{registrations: []Registration{
		{Name: "ubuntu", BasePath: t.TempDir()},
	}}, allocations).Inspect(context.Background())

	if report.Distributions[0].VHDX.PathAvailable || len(allocations.calls) != 0 {
		t.Fatalf("case-mismatched registration authorized a path: %#v", report)
	}
}

func TestInspectFailsClosedOnMalformedOptionalOutputAndSafeMeasurementError(t *testing.T) {
	base := t.TempDir()
	disk := filepath.Join(base, "ext4.vhdx")
	runner := &fakeRunner{
		outputs: map[string][]byte{
			"--list --quiet":           []byte("Ubuntu\n"),
			"--list --running --quiet": {0xff, 0xfe, 0x00},
			"--list --verbose":         {0xff, 0xfe, 0x00, 0xd8},
		},
	}
	allocations := &fakeAllocationSource{errs: map[string]error{disk: errors.New("registered ext4.vhdx is a reparse point")}}
	report := NewInspector(runner, fakeRegistrationSource{registrations: []Registration{{Name: "Ubuntu", BasePath: base}}}, allocations).Inspect(context.Background())
	distribution := report.Distributions[0]
	if distribution.State != core.WSLStateUnknown || distribution.VersionAvailable || distribution.VHDX.PathAvailable {
		t.Fatalf("malformed output or unsafe path did not fail closed: %#v", distribution)
	}
	if distribution.VHDX.PhysicalSize.Kind != core.MeasurementUnavailable || len(report.Warnings) < 3 {
		t.Fatalf("warnings or unavailable measurement missing: %#v", report)
	}
	assertReadOnlyCommands(t, runner.calls)
}

func TestInspectRejectsMalformedMandatoryOutput(t *testing.T) {
	runner := &fakeRunner{outputs: map[string][]byte{"--list --quiet": {0xff, 0xfe, 0x41}}}
	report := NewInspector(runner, fakeRegistrationSource{}, &fakeAllocationSource{}).Inspect(context.Background())
	if report.Available || !strings.Contains(report.Message, "malformed") || len(runner.calls) != 1 {
		t.Fatalf("mandatory malformed output was accepted: %#v", report)
	}
}

func TestInspectDecodesRawCommandDiagnostics(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string][]byte{"--list --quiet": encodeUTF16("Truy cập bị từ chối", binary.LittleEndian, true)},
		errors:  map[string]error{"--list --quiet": errors.New("exit status 1")},
	}
	report := NewInspector(runner, fakeRegistrationSource{}, &fakeAllocationSource{}).Inspect(context.Background())
	if !strings.Contains(report.Message, "Truy cập bị từ chối") {
		t.Fatalf("UTF-16 command diagnostic was not decoded: %q", report.Message)
	}
}

func TestInspectDoesNotInventRegistrationOrStartDistribution(t *testing.T) {
	runner := standardRunner("Imported Distro")
	allocations := &fakeAllocationSource{}
	report := NewInspector(runner, fakeRegistrationSource{}, allocations).Inspect(context.Background())
	if len(report.Distributions) != 1 || report.Distributions[0].VHDX.PathAvailable || report.Distributions[0].VHDX.Path != "" {
		t.Fatalf("missing registration metadata was invented: %#v", report)
	}
	if !strings.Contains(report.Message, "no backing VHDX file was safely measured") {
		t.Fatalf("report overstated backing-file evidence: %q", report.Message)
	}
	if len(allocations.calls) != 0 {
		t.Fatalf("allocation metadata was accessed without an evidence-backed path: %#v", allocations.calls)
	}
	assertReadOnlyCommands(t, runner.calls)
}

func TestInspectHandlesMissingCLI(t *testing.T) {
	report := NewInspector(&fakeRunner{lookPathErr: os.ErrNotExist}, fakeRegistrationSource{}, &fakeAllocationSource{}).Inspect(context.Background())
	if report.CLIAvailable || report.Available || len(report.Distributions) != 0 {
		t.Fatalf("missing CLI report is unsafe: %#v", report)
	}
}

func TestDecodeWindowsOutputStrictEncodings(t *testing.T) {
	text := "Ubuntu 東京 🐧\r\n"
	tests := []struct {
		name    string
		input   []byte
		want    string
		wantErr bool
	}{
		{name: "UTF-8", input: []byte(text), want: text},
		{name: "UTF-8 BOM", input: append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...), want: text},
		{name: "UTF-16LE BOM CJK and surrogate", input: encodeUTF16(text, binary.LittleEndian, true), want: text},
		{name: "UTF-16BE BOM CJK and surrogate", input: encodeUTF16(text, binary.BigEndian, true), want: text},
		{name: "BOM-less UTF-16LE ASCII", input: encodeUTF16("Ubuntu\r\n", binary.LittleEndian, false), want: "Ubuntu\r\n"},
		{name: "BOM-less UTF-16BE ASCII", input: encodeUTF16("Ubuntu\r\n", binary.BigEndian, false), want: "Ubuntu\r\n"},
		{name: "odd UTF-16", input: []byte{0xff, 0xfe, 0x41}, wantErr: true},
		{name: "unpaired high surrogate", input: []byte{0xff, 0xfe, 0x00, 0xd8}, wantErr: true},
		{name: "unpaired low surrogate", input: []byte{0xff, 0xfe, 0x00, 0xdc}, wantErr: true},
		{name: "malformed surrogate pair", input: []byte{0xff, 0xfe, 0x00, 0xd8, 0x41, 0x00}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodeWindowsOutput(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("decodeWindowsOutput() = %q, want error", got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("decodeWindowsOutput() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestParseUTF16WSLOutputUsesExactNames(t *testing.T) {
	output := encodeUTF16("  NAME              STATE      VERSION\r\n* Ubuntu Dev        Running    2\r\n  Ubuntu            Stopped    1\r\n", binary.LittleEndian, true)
	versions, err := parseVerboseVersions(output, []string{"Ubuntu", "Ubuntu Dev"})
	if err != nil || versions["Ubuntu Dev"] != 2 || versions["Ubuntu"] != 1 {
		t.Fatalf("unexpected parsed versions: %#v, %v", versions, err)
	}
	names, ambiguous, err := parseQuietNames(encodeUTF16("Ubuntu\r\nUbuntu\r\nubuntu\r\n", binary.LittleEndian, true))
	if err != nil || len(names) != 2 || names[0] != "Ubuntu" || names[1] != "ubuntu" || len(ambiguous) != 1 {
		t.Fatalf("unexpected exact quiet names or ambiguity: %#v, %#v, %v", names, ambiguous, err)
	}
}

func standardRunner(name string) *fakeRunner {
	return &fakeRunner{outputs: map[string][]byte{
		"--list --quiet":           []byte(name + "\n"),
		"--list --running --quiet": nil,
		"--list --verbose":         []byte("NAME STATE VERSION\n" + name + " Stopped 2\n"),
	}}
}

func warningsContain(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}

func assertReadOnlyCommands(t *testing.T, calls []string) {
	t.Helper()
	want := []string{"--list --quiet", "--list --running --quiet", "--list --verbose"}
	if len(calls) != len(want) {
		t.Fatalf("WSL command count = %d, want %d: %#v", len(calls), len(want), calls)
	}
	for index := range want {
		if calls[index] != want[index] {
			t.Fatalf("WSL command %d = %q, want %q", index, calls[index], want[index])
		}
		lower := strings.ToLower(calls[index])
		for _, prohibited := range []string{"shutdown", "terminate", "manage", "mount", "unmount", "export", "import", "unregister", "set-sparse", "compact", "optimize"} {
			if strings.Contains(lower, prohibited) {
				t.Fatalf("prohibited WSL command observed: %q", calls[index])
			}
		}
	}
}

func encodeUTF16(value string, order binary.ByteOrder, withBOM bool) []byte {
	units := utf16.Encode([]rune(value))
	offset := 0
	if withBOM {
		offset = 2
	}
	encoded := make([]byte, offset+len(units)*2)
	if withBOM {
		if order == binary.LittleEndian {
			encoded[0], encoded[1] = 0xff, 0xfe
		} else {
			encoded[0], encoded[1] = 0xfe, 0xff
		}
	}
	for index, unit := range units {
		order.PutUint16(encoded[offset+index*2:], unit)
	}
	return encoded
}

func TestInspectPermanentlyRejectsDuplicateCLIIdentity(t *testing.T) {
	base := t.TempDir()
	runner := standardRunner("Ubuntu")
	runner.outputs["--list --quiet"] = []byte("Ubuntu\nUbuntu\nUbuntu\n")
	allocations := &fakeAllocationSource{allocations: map[string]FileAllocation{
		filepath.Join(base, "ext4.vhdx"): {AllocatedBytes: 4096, EndOfFileBytes: 8192},
	}}
	report := NewInspector(runner, fakeRegistrationSource{registrations: []Registration{{Name: "Ubuntu", BasePath: base}}}, allocations).Inspect(context.Background())

	if len(report.Distributions) != 1 || report.Distributions[0].VHDX.PathAvailable || len(allocations.calls) != 0 {
		t.Fatalf("duplicate CLI identity authorized a registry path: %#v; calls=%#v", report, allocations.calls)
	}
	if !warningsContain(report.Warnings, "Duplicate WSL CLI identity permanently disabled") {
		t.Fatalf("missing CLI ambiguity warning: %#v", report.Warnings)
	}
}

func TestInspectRejectsDuplicateVerboseIdentityAndSkipsVHDX(t *testing.T) {
	base := t.TempDir()
	runner := standardRunner("Ubuntu")
	runner.outputs["--list --verbose"] = []byte("NAME STATE VERSION\nUbuntu Stopped 1\nUbuntu Stopped 2\n")
	allocations := &fakeAllocationSource{allocations: map[string]FileAllocation{
		filepath.Join(base, "ext4.vhdx"): {AllocatedBytes: 4096, EndOfFileBytes: 8192},
	}}
	report := NewInspector(runner, fakeRegistrationSource{registrations: []Registration{{Name: "Ubuntu", BasePath: base}}}, allocations).Inspect(context.Background())

	if report.Distributions[0].VersionAvailable || report.Distributions[0].VHDX.PathAvailable || len(allocations.calls) != 0 {
		t.Fatalf("duplicate verbose identity influenced VHDX measurement: %#v; calls=%#v", report, allocations.calls)
	}
	if !warningsContain(report.Warnings, "duplicate identity") {
		t.Fatalf("missing verbose ambiguity warning: %#v", report.Warnings)
	}
}
