//go:build windows
// +build windows

package mmap

import (
	"os"
	"syscall"
	"unsafe"
)

type MMap struct {
	data []byte
	file *os.File
	h    syscall.Handle
}

func Open(filename string, size int64) (*MMap, error) {
	file, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	h, err := syscall.CreateFileMapping(syscall.Handle(file.Fd()), nil, syscall.PAGE_READWRITE, uint32(size>>32), uint32(size), nil)
	if err != nil {
		file.Close()
		return nil, err
	}

	addr, err := syscall.MapViewOfFile(h, syscall.FILE_MAP_WRITE, 0, 0, uintptr(size))
	if err != nil {
		syscall.CloseHandle(h)
		file.Close()
		return nil, err
	}

	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), int(size))

	return &MMap{data: data, file: file, h: h}, nil
}

func (m *MMap) Close() error {
	if len(m.data) > 0 {
		syscall.UnmapViewOfFile(uintptr(unsafe.Pointer(&m.data[0])))
	}
	if m.h != 0 {
		syscall.CloseHandle(m.h)
	}
	return m.file.Close()
}

func (m *MMap) Data() []byte {
	return m.data
}

func (m *MMap) Prefetch(offset, size int) error {
	return nil
}
