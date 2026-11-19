package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
)

type mediaDLL struct {
	newHandle func() uintptr
	delete    func(handle uintptr)
	open      func(handle uintptr, path *uint16) uintptr
	get       func(handle, kind, stream uintptr, field *uint16, info, search uintptr) *uint16
}

var loadMediaDLL = sync.OnceValues(func() (*mediaDLL, error) {
	compressed, err := gzip.NewReader(bytes.NewReader(compressedMediaDLL))
	if err != nil {
		return nil, err
	}
	defer compressed.Close()
	contents, err := io.ReadAll(compressed)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(contents)
	path, err := filepath.Abs(filepath.Join(dataDir(), fmt.Sprintf("MediaInfo-%x.dll", sum[:8])))
	if err != nil {
		return nil, err
	}
	if disk, _ := os.ReadFile(path); !bytes.Equal(disk, contents) {
		if err := writeMediaDLL(path, contents); err != nil {
			return nil, err
		}
	}
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return nil, err
	}
	api := new(mediaDLL)
	for _, entry := range []struct {
		name   string
		target any
	}{
		{"MediaInfo_New", &api.newHandle},
		{"MediaInfo_Delete", &api.delete},
		{"MediaInfo_Open", &api.open},
		{"MediaInfo_Get", &api.get},
	} {
		proc, err := dll.FindProc(entry.name)
		if err != nil {
			_ = dll.Release()
			return nil, err
		}
		purego.RegisterFunc(entry.target, proc.Addr())
	}
	return api, nil
})

func writeMediaDLL(path string, contents []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "mediainfo-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(contents)
	closeErr := tmp.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		// A concurrent launch may have installed and loaded the same library.
		if disk, _ := os.ReadFile(path); !bytes.Equal(disk, contents) {
			return err
		}
	}
	return nil
}

func readMediaProperties(path string, image bool) (MediaProperties, error) {
	var props MediaProperties
	api, err := loadMediaDLL()
	if err != nil {
		return props, err
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return props, nil
	}
	handle := api.newHandle()
	if handle == 0 {
		return props, fmt.Errorf("无法启动 MediaInfo")
	}
	defer api.delete(handle)
	if api.open(handle, name) == 0 {
		return props, nil
	}
	const streamGeneral, streamVideo, streamImage = 0, 1, 5
	if image {
		props.Width = int(api.number(handle, streamImage, "Width"))
		props.Height = int(api.number(handle, streamImage, "Height"))
		if props.Width == 0 || props.Height == 0 {
			// Some animated images are reported as a video stream.
			props.Width = int(api.number(handle, streamVideo, "Width"))
			props.Height = int(api.number(handle, streamVideo, "Height"))
		}
		if math.Mod(math.Abs(api.number(handle, streamImage, "Rotation")), 180) == 90 {
			props.Width, props.Height = props.Height, props.Width
		}
	} else {
		ms := api.number(handle, streamGeneral, "Duration")
		if ms == 0 {
			ms = api.number(handle, streamVideo, "Duration")
		}
		props.DurationSeconds = ms / 1000
	}
	return props, nil
}

func (api *mediaDLL) number(handle, kind uintptr, field string) float64 {
	key, _ := syscall.UTF16PtrFromString(field)
	const infoText, infoName = 1, 0
	value := api.get(handle, kind, 0, key, infoText, infoName)
	if value == nil {
		return 0
	}
	text := syscall.UTF16ToString(unsafe.Slice(value, 32))
	n, _ := strconv.ParseFloat(text, 64)
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
		return 0
	}
	return n
}
