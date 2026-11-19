//go:build windows

package main

import (
	"os"
	"syscall"
	"time"
)

// creationTime 取 Windows 的文件创建时间。
func creationTime(fi os.FileInfo) (time.Time, bool) {
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, d.CreationTime.Nanoseconds()), true
}
