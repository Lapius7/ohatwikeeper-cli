//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode = kernel32.NewProc("GetConsoleMode")
	setConsoleMode = kernel32.NewProc("SetConsoleMode")
	getBufferInfo  = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

// enableVT はWindowsのコンソールでANSIエスケープ(色)を有効にする
func enableVT() {
	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return
	}
	var mode uint32
	if r, _, _ := getConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode))); r == 0 {
		return
	}
	setConsoleMode.Call(uintptr(h), uintptr(mode|0x0004)) // ENABLE_VIRTUAL_TERMINAL_PROCESSING
}

func ttyWidth() int {
	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return 0
	}
	type coord struct{ X, Y int16 }
	var info struct {
		Size, Cursor coord
		Attr         uint16
		L, T, R, B   int16
		Max          coord
	}
	if r, _, _ := getBufferInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&info))); r == 0 {
		return 0
	}
	return int(info.R-info.L) + 1
}
