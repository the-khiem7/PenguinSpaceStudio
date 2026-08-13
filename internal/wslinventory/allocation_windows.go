//go:build windows

package wslinventory

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type systemAllocationSource struct{}

type fileAttributeTagInfo struct {
	FileAttributes uint32
	ReparseTag     uint32
}

type fileStandardInfo struct {
	AllocationSize int64
	EndOfFile      int64
	NumberOfLinks  uint32
	DeletePending  byte
	Directory      byte
}

func (systemAllocationSource) MeasureAllocation(path string) (FileAllocation, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return FileAllocation{}, fmt.Errorf("encode VHDX path: %w", err)
	}
	handle, err := windows.CreateFile(
		pathPointer,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return FileAllocation{}, err
	}
	defer windows.CloseHandle(handle)

	var attributes fileAttributeTagInfo
	if err := windows.GetFileInformationByHandleEx(
		handle,
		windows.FileAttributeTagInfo,
		(*byte)(unsafe.Pointer(&attributes)),
		uint32(unsafe.Sizeof(attributes)),
	); err != nil {
		return FileAllocation{}, fmt.Errorf("read VHDX handle attributes: %w", err)
	}
	if attributes.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return FileAllocation{}, errors.New("registered ext4.vhdx is a reparse point")
	}
	if attributes.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return FileAllocation{}, errors.New("registered ext4.vhdx is not a regular file")
	}

	var standard fileStandardInfo
	if err := windows.GetFileInformationByHandleEx(
		handle,
		windows.FileStandardInfo,
		(*byte)(unsafe.Pointer(&standard)),
		uint32(unsafe.Sizeof(standard)),
	); err != nil {
		return FileAllocation{}, fmt.Errorf("read VHDX allocation metadata: %w", err)
	}
	if standard.Directory != 0 {
		return FileAllocation{}, errors.New("registered ext4.vhdx is not a regular file")
	}
	if standard.AllocationSize < 0 || standard.EndOfFile < 0 {
		return FileAllocation{}, errors.New("registered ext4.vhdx reported invalid file sizes")
	}
	return FileAllocation{
		AllocatedBytes: uint64(standard.AllocationSize),
		EndOfFileBytes: uint64(standard.EndOfFile),
	}, nil
}
