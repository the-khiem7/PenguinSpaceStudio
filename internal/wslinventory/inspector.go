package wslinventory

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/the-khiem7/PenguinSpaceStudio/internal/core"
	"github.com/the-khiem7/PenguinSpaceStudio/internal/providers/common"
)

type Registration struct {
	Name     string
	BasePath string
}

type RegistrationSource interface {
	List(context.Context) ([]Registration, error)
}

type FileAllocation struct {
	AllocatedBytes uint64
	EndOfFileBytes uint64
}

type AllocationSource interface {
	MeasureAllocation(string) (FileAllocation, error)
}

type Inspector struct {
	runner        common.RawCommandRunner
	registrations RegistrationSource
	allocations   AllocationSource
}

func NewSystemInspector() *Inspector {
	return NewInspector(common.SystemRunner{}, newSystemRegistrationSource(), systemAllocationSource{})
}

func NewInspector(runner common.RawCommandRunner, registrations RegistrationSource, allocations AllocationSource) *Inspector {
	return &Inspector{runner: runner, registrations: registrations, allocations: allocations}
}

func (i *Inspector) Inspect(ctx context.Context) core.WSLAwareness {
	report := core.WSLAwareness{InspectedAt: nowUTC()}
	executable, err := i.runner.LookPath("wsl.exe")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			report.Message = "WSL CLI was not found. No distributions or virtual disks were inspected."
			return report
		}
		report.Message = fmt.Sprintf("WSL CLI lookup failed: %v", err)
		return report
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		report.Message = fmt.Sprintf("WSL CLI path could not be resolved: %v", err)
		return report
	}
	report.CLIAvailable = true
	report.ExecutablePath = executable

	allOutput, runErr := i.runner.RunRaw(ctx, executable, "--list", "--quiet")
	if runErr != nil {
		report.Message = commandFailureMessage("installed distributions could not be listed", allOutput, runErr)
		return report
	}
	names, ambiguousNames, err := parseQuietNames(allOutput)
	if err != nil {
		report.Message = fmt.Sprintf("WSL CLI is available, but installed distribution output was malformed: %v", err)
		return report
	}
	report.Available = true
	for name := range ambiguousNames {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Duplicate WSL CLI identity permanently disabled the VHDX path for %q.", name))
	}
	if len(names) == 0 {
		report.Message = "WSL is available, but no registered distributions were reported."
		return report
	}

	runningOutput, runningRunErr := i.runner.RunRaw(ctx, executable, "--list", "--running", "--quiet")
	running := make(map[string]struct{})
	runningTrusted := runningRunErr == nil
	if runningRunErr != nil {
		report.Warnings = append(report.Warnings, commandFailureMessage("running distribution state was unavailable", runningOutput, runningRunErr))
	} else {
		runningNames, runningAmbiguous, decodeErr := parseQuietNames(runningOutput)
		if decodeErr != nil {
			runningTrusted = false
			report.Warnings = append(report.Warnings, fmt.Sprintf("Running distribution state output was malformed: %v", decodeErr))
		} else if len(runningAmbiguous) > 0 {
			runningTrusted = false
			report.Warnings = append(report.Warnings, "Running distribution state contained duplicate identities; all state values were disabled.")
		} else {
			for _, name := range runningNames {
				running[name] = struct{}{}
			}
		}
	}

	verboseOutput, verboseRunErr := i.runner.RunRaw(ctx, executable, "--list", "--verbose")
	versions := make(map[string]uint32)
	if verboseRunErr != nil {
		report.Warnings = append(report.Warnings, commandFailureMessage("distribution versions were unavailable", verboseOutput, verboseRunErr))
	} else {
		var decodeErr error
		versions, decodeErr = parseVerboseVersions(verboseOutput, names)
		if decodeErr != nil {
			versions = make(map[string]uint32)
			report.Warnings = append(report.Warnings, fmt.Sprintf("WSL distribution version output was malformed: %v", decodeErr))
		}
	}

	registrations, registrationErr := i.registrations.List(ctx)
	registrationByName := make(map[string]Registration)
	if registrationErr != nil {
		registrations = nil
		report.Warnings = append(report.Warnings, fmt.Sprintf("WSL registration metadata was incomplete; every registry-derived VHDX path was disabled: %v", registrationErr))
	}
	for _, registration := range registrations {
		if registration.Name == "" || strings.TrimSpace(registration.Name) != registration.Name {
			report.Warnings = append(report.Warnings, "A WSL registration had an invalid distribution name and was ignored.")
			continue
		}
		if _, ambiguous := ambiguousNames[registration.Name]; ambiguous {
			continue
		}
		if _, duplicate := registrationByName[registration.Name]; duplicate {
			delete(registrationByName, registration.Name)
			ambiguousNames[registration.Name] = struct{}{}
			report.Warnings = append(report.Warnings, fmt.Sprintf("Duplicate WSL registration metadata permanently disabled the VHDX path for %q.", registration.Name))
			continue
		}
		registrationByName[registration.Name] = registration
	}

	sort.Slice(names, func(left, right int) bool {
		leftFolded, rightFolded := strings.ToLower(names[left]), strings.ToLower(names[right])
		if leftFolded == rightFolded {
			return names[left] < names[right]
		}
		return leftFolded < rightFolded
	})
	report.Distributions = make([]core.WSLDistribution, 0, len(names))
	measuredVHDX := 0
	for _, name := range names {
		distribution := core.WSLDistribution{
			Name:  name,
			State: core.WSLStateUnknown,
			VHDX:  unavailableVHDX("Backing VHDX metadata is unavailable for this distribution."),
		}
		if runningTrusted {
			distribution.State = core.WSLStateStopped
			if _, isRunning := running[name]; isRunning {
				distribution.State = core.WSLStateRunning
			}
		}
		if version, ok := versions[name]; ok {
			distribution.Version = version
			distribution.VersionAvailable = true
		} else {
			report.Warnings = append(report.Warnings, fmt.Sprintf("WSL version was unavailable for %q.", name))
		}

		if !distribution.VersionAvailable {
			distribution.VHDX = unavailableVHDX("Backing VHDX measurement requires unambiguous WSL 2 version evidence.")
			report.Distributions = append(report.Distributions, distribution)
			continue
		}
		if distribution.Version == 1 {
			distribution.VHDX = unavailableVHDX("WSL 1 distributions do not use a WSL 2 ext4.vhdx backing disk.")
			report.Distributions = append(report.Distributions, distribution)
			continue
		}
		registration, registered := registrationByName[name]
		if !registered {
			distribution.VHDX = unavailableVHDX("No exact, unambiguous current-user WSL registration metadata was available.")
			report.Warnings = append(report.Warnings, fmt.Sprintf("Backing VHDX path metadata was unavailable for %q.", name))
			report.Distributions = append(report.Distributions, distribution)
			continue
		}
		distribution.VHDX = i.inspectVHDX(registration.BasePath)
		if distribution.VHDX.PathAvailable {
			measuredVHDX++
		} else {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Backing VHDX was not measurable for %q: %s", name, distribution.VHDX.Message))
		}
		report.Distributions = append(report.Distributions, distribution)
	}
	if measuredVHDX == 0 {
		report.Message = "WSL distributions were listed without starting or stopping them; no backing VHDX file was safely measured."
	} else {
		report.Message = fmt.Sprintf("WSL distributions were listed and allocated host bytes were safely measured for %d backing VHDX file(s), without starting, stopping, mounting, or modifying them.", measuredVHDX)
	}
	return report
}

func (i *Inspector) inspectVHDX(basePath string) core.WSLVHDXObservation {
	observation := unavailableVHDX("The current-user WSL registration did not provide an absolute base path.")
	if basePath == "" || strings.TrimSpace(basePath) != basePath || !filepath.IsAbs(basePath) {
		return observation
	}
	candidate := filepath.Clean(filepath.Join(basePath, "ext4.vhdx"))
	observation.Path = candidate
	allocation, err := i.allocations.MeasureAllocation(candidate)
	if errors.Is(err, os.ErrNotExist) {
		observation.Message = "The registered ext4.vhdx path does not exist."
		return observation
	}
	if err != nil {
		observation.Message = fmt.Sprintf("The registered ext4.vhdx path could not be measured safely: %v", err)
		return observation
	}
	observation.PathAvailable = true
	observation.PhysicalSize = core.Measurement{Bytes: allocation.AllocatedBytes, Kind: core.MeasurementMeasuredPhysical}
	observation.Message = "Allocated host bytes are measured from the non-reparse file handle. Logical usage and compactable bytes remain unavailable."
	return observation
}

func unavailableVHDX(message string) core.WSLVHDXObservation {
	return core.WSLVHDXObservation{
		PhysicalSize: core.Measurement{Kind: core.MeasurementUnavailable},
		LogicalUsage: core.Measurement{Kind: core.MeasurementUnavailable},
		Compactable:  core.Measurement{Kind: core.MeasurementUnavailable},
		Message:      message,
	}
}

func parseQuietNames(output []byte) ([]string, map[string]struct{}, error) {
	decoded, err := decodeWindowsOutput(output)
	if err != nil {
		return nil, nil, err
	}
	seen := make(map[string]struct{})
	ambiguous := make(map[string]struct{})
	var names []string
	for _, line := range strings.Split(decoded, "\n") {
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*"))
		if name == "" {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			ambiguous[name] = struct{}{}
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names, ambiguous, nil
}

func parseVerboseVersions(output []byte, names []string) (map[string]uint32, error) {
	decoded, err := decodeWindowsOutput(output)
	if err != nil {
		return nil, err
	}
	orderedNames := append([]string(nil), names...)
	sort.Slice(orderedNames, func(left, right int) bool { return len(orderedNames[left]) > len(orderedNames[right]) })
	versions := make(map[string]uint32)
	seenRows := make(map[string]struct{})
	for _, line := range strings.Split(decoded, "\n") {
		row := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*"))
		for _, name := range orderedNames {
			if !strings.HasPrefix(row, name) || len(row) == len(name) || !isSpace(row[len(name)]) {
				continue
			}
			if _, duplicate := seenRows[name]; duplicate {
				return nil, fmt.Errorf("verbose output contained duplicate identity %q", name)
			}
			seenRows[name] = struct{}{}
			fields := strings.Fields(row[len(name):])
			if len(fields) == 0 {
				break
			}
			version, parseErr := strconv.ParseUint(fields[len(fields)-1], 10, 32)
			if parseErr == nil && (version == 1 || version == 2) {
				versions[name] = uint32(version)
			}
			break
		}
	}
	return versions, nil
}

func commandFailureMessage(action string, output []byte, runErr error) string {
	message := fmt.Sprintf("WSL CLI is available, but %s: %v", action, runErr)
	if len(output) == 0 {
		return message
	}
	decoded, decodeErr := decodeWindowsOutput(output)
	if decodeErr != nil {
		return fmt.Sprintf("%s (diagnostic output was malformed: %v)", message, decodeErr)
	}
	if diagnostic := strings.TrimSpace(decoded); diagnostic != "" {
		return fmt.Sprintf("%s: %s", message, diagnostic)
	}
	return message
}

// decodeWindowsOutput accepts UTF-8 (with an optional BOM), BOM-tagged UTF-16LE/BE,
// and BOM-less UTF-16 only when alternating NUL bytes make the byte order unambiguous.
func decodeWindowsOutput(data []byte) (string, error) {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		data = data[3:]
	}
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe {
		return decodeUTF16(data[2:], binary.LittleEndian)
	}
	if len(data) >= 2 && data[0] == 0xfe && data[1] == 0xff {
		return decodeUTF16(data[2:], binary.BigEndian)
	}
	if utf8.Valid(data) && !containsNUL(data) {
		return string(data), nil
	}
	if order, ok := detectBOMlessUTF16(data); ok {
		return decodeUTF16(data, order)
	}
	return "", errors.New("output is neither valid UTF-8 nor unambiguous UTF-16")
}

func detectBOMlessUTF16(data []byte) (binary.ByteOrder, bool) {
	if len(data) == 0 || len(data)%2 != 0 {
		return nil, false
	}
	var evenNUL, oddNUL int
	for index, value := range data {
		if value != 0 {
			continue
		}
		if index%2 == 0 {
			evenNUL++
		} else {
			oddNUL++
		}
	}
	pairs := len(data) / 2
	if oddNUL > 0 && evenNUL == 0 && oddNUL*2 >= pairs {
		return binary.LittleEndian, true
	}
	if evenNUL > 0 && oddNUL == 0 && evenNUL*2 >= pairs {
		return binary.BigEndian, true
	}
	return nil, false
}

func decodeUTF16(data []byte, order binary.ByteOrder) (string, error) {
	if len(data)%2 != 0 {
		return "", errors.New("UTF-16 output has an odd byte length")
	}
	runes := make([]rune, 0, len(data)/2)
	for offset := 0; offset < len(data); offset += 2 {
		unit := order.Uint16(data[offset : offset+2])
		switch {
		case unit >= 0xd800 && unit <= 0xdbff:
			if offset+4 > len(data) {
				return "", errors.New("UTF-16 output ends with an unpaired high surrogate")
			}
			low := order.Uint16(data[offset+2 : offset+4])
			if low < 0xdc00 || low > 0xdfff {
				return "", errors.New("UTF-16 output contains a malformed surrogate pair")
			}
			runes = append(runes, rune(0x10000+(uint32(unit)-0xd800)<<10+(uint32(low)-0xdc00)))
			offset += 2
		case unit >= 0xdc00 && unit <= 0xdfff:
			return "", errors.New("UTF-16 output contains an unpaired low surrogate")
		default:
			runes = append(runes, rune(unit))
		}
	}
	return string(runes), nil
}

func containsNUL(data []byte) bool {
	return strings.IndexByte(string(data), 0) >= 0
}

func isSpace(value byte) bool {
	return value == ' ' || value == '\t'
}

var nowUTC = func() time.Time {
	return time.Now().UTC()
}
