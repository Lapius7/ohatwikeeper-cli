//go:build !windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

func enableVT() {}

func ttyWidth() int {
	var ws struct{ Row, Col, X, Y uint16 }
	for _, f := range []*os.File{os.Stdout, os.Stderr, os.Stdin} {
		_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
		if e == 0 && ws.Col > 0 {
			return int(ws.Col)
		}
	}
	return 0
}
