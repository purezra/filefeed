//go:build windows

package main

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenClipboard    = modUser32.NewProc("OpenClipboard")
	procEmptyClipboard   = modUser32.NewProc("EmptyClipboard")
	procCloseClipboard   = modUser32.NewProc("CloseClipboard")
	procSetClipboardData = modUser32.NewProc("SetClipboardData")
	procGlobalAlloc      = modKernel32.NewProc("GlobalAlloc")
	procGlobalLock       = modKernel32.NewProc("GlobalLock")
	procGlobalUnlock     = modKernel32.NewProc("GlobalUnlock")
	procGlobalFree       = modKernel32.NewProc("GlobalFree")
)

const (
	cfHDrop      = 15
	gmemMoveable = 0x0002
)

// dropFiles 是 CF_HDROP 剪贴板数据的头部（DROPFILES 结构）。
type dropFiles struct {
	pFiles uint32
	ptX    int32
	ptY    int32
	fNC    int32
	fWide  int32
}

// copyFilesToClipboard 把文件放进剪贴板（CF_HDROP 格式），
// 之后可以在资源管理器里直接 Ctrl+V 粘贴出文件本身。
func copyFilesToClipboard(paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	// DROPFILES + UTF-16 路径（每段以 \0 结尾）+ 结尾双 \0
	var buf []uint16
	for _, p := range paths {
		buf = append(buf, utf16.Encode([]rune(p))...)
		buf = append(buf, 0)
	}
	buf = append(buf, 0)

	hdrSize := int(unsafe.Sizeof(dropFiles{}))
	total := uintptr(hdrSize + len(buf)*2)

	h, _, allocErr := procGlobalAlloc.Call(gmemMoveable, total)
	if h == 0 {
		return allocErr
	}
	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		procGlobalFree.Call(h)
		return allocErr
	}

	hdr := (*dropFiles)(unsafe.Pointer(ptr))
	hdr.pFiles = uint32(hdrSize)
	hdr.fWide = 1
	dest := unsafe.Slice((*uint16)(unsafe.Pointer(ptr+uintptr(hdrSize))), len(buf))
	copy(dest, buf)
	procGlobalUnlock.Call(h)

	if r, _, openErr := procOpenClipboard.Call(0); r == 0 {
		procGlobalFree.Call(h)
		return openErr
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()
	if r, _, setErr := procSetClipboardData.Call(cfHDrop, h); r == 0 {
		procGlobalFree.Call(h)
		return setErr
	}
	// 成功后内存归系统所有，不能再释放
	return nil
}
