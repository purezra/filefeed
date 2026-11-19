package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestImageFilters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "图片.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 640, 360))); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		filter ImageFilter
		match  bool
	}{
		{"ratio and exact dimensions", ImageFilter{Ratio: "16:9", Width: 640, Height: 360, ResolutionMode: "exact"}, true},
		{"portrait", ImageFilter{Ratio: "竖图"}, false},
		{"minimum width", ImageFilter{Width: 720, ResolutionMode: "min"}, false},
		{"maximum height", ImageFilter{Height: 400, ResolutionMode: "max"}, true},
		{"size only", ImageFilter{MaxMB: 0.00001}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.filter.Exts = []string{"png"}
			filters, err := newMediaFilters(test.filter, VideoFilter{})
			if err != nil {
				t.Fatal(err)
			}
			match, missing, props, err := filters.match("png", path, info)
			if err != nil || missing || match != test.match {
				t.Fatalf("match=%v missing=%v properties=%+v error=%v", match, missing, props, err)
			}
		})
	}
}

func TestSVGViewBoxRatio(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vector.svg")
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 9"/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	image := ImageFilter{Exts: []string{"svg"}, Ratio: "16:9"}
	filters, err := newMediaFilters(image, VideoFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if match, missing, _, err := filters.match("svg", path, info); !match || missing || err != nil {
		t.Fatalf("viewBox should match its aspect ratio: %v %v %v", match, missing, err)
	}
	filters.picture.Width = 16
	if match, missing, _, err := filters.match("svg", path, info); match || !missing || err != nil {
		t.Fatalf("viewBox is not a pixel resolution: %v %v %v", match, missing, err)
	}
}
