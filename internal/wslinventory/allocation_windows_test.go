//go:build windows

package wslinventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSystemAllocationSourceUsesSparseAllocationInsteadOfEOF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ext4.vhdx")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	var returned uint32
	if err := windows.DeviceIoControl(windows.Handle(file.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &returned, nil); err != nil {
		_ = file.Close()
		t.Skipf("filesystem does not support sparse-file test: %v", err)
	}
	const logicalSize = 64 << 20
	if err := file.Truncate(logicalSize); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	allocation, err := (systemAllocationSource{}).MeasureAllocation(path)
	if err != nil {
		t.Fatal(err)
	}
	if allocation.EndOfFileBytes != logicalSize {
		t.Fatalf("EOF = %d, want %d", allocation.EndOfFileBytes, logicalSize)
	}
	if allocation.AllocatedBytes >= allocation.EndOfFileBytes {
		t.Fatalf("allocated bytes = %d, EOF = %d; physical measurement did not preserve sparse allocation", allocation.AllocatedBytes, allocation.EndOfFileBytes)
	}
}

func TestSystemAllocationSourceRejectsReparsePoint(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.vhdx")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "ext4.vhdx")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation is unavailable for this Windows test account: %v", err)
	}

	_, err := (systemAllocationSource{}).MeasureAllocation(link)
	if err == nil || !strings.Contains(err.Error(), "reparse point") {
		t.Fatalf("MeasureAllocation(reparse point) error = %v, want explicit rejection", err)
	}
}
