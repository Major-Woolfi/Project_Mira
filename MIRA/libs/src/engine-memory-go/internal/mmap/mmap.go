//go:build linux || darwin
// +build linux darwin

package mmap

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

type MMap struct {
	data []byte
	file *os.File
}

func Open(filename string, size int64) (*MMap, error) {
	file, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	data, err := unix.Mmap(int(file.Fd()), 0, int(size), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, err
	}

	return &MMap{data: data, file: file}, nil
}

func (m *MMap) Close() error {
	if err := unix.Munmap(m.data); err != nil {
		return err
	}
	return m.file.Close()
}

func (m *MMap) Data() []byte {
	return m.data
}

func (m *MMap) Prefetch(offset, size int) error {
	_, _, errno := unix.Syscall(unix.SYS_MADVISE, uintptr(unsafe.Pointer(&m.data[offset])), uintptr(size), uintptr(unix.MADV_WILLNEED))
	if errno != 0 {
		return errno
	}
	return nil
}
