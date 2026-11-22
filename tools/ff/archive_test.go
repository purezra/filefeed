package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDailyCindyArchive(t *testing.T) {
	root := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", root)
		t.Setenv("APPDATA", root)
		t.Setenv("LOCALAPPDATA", root)
	} else {
		t.Setenv("HOME", root)
		t.Setenv("XDG_CONFIG_HOME", root)
		t.Setenv("XDG_CACHE_HOME", root)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	dir = filepath.Join(dir, "CindyGlobal", "pi-agent-home", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 22, 0, 0, 0, 0, time.Local)
	history := func(id string, offsets ...int) {
		t.Helper()
		start := day.AddDate(0, 0, offsets[0])
		var log strings.Builder
		fmt.Fprintf(&log, `{"type":"session","version":3,"id":%q,"timestamp":%q,"cwd":"project"}`+"\n", id, start.Format(time.RFC3339Nano))
		for i, offset := range offsets {
			parent := "null"
			if i > 0 {
				parent = fmt.Sprintf(`"msg%d"`, i-1)
			}
			at := day.AddDate(0, 0, offset)
			fmt.Fprintf(&log, `{"type":"message","id":"msg%d","parentId":%s,"timestamp":%q,"message":{"role":"user","content":"fixture day %d"}}`+"\n", i, parent, at.Format(time.RFC3339Nano), offset)
		}
		name := start.UTC().Format("2006-01-02T15-04-05-000Z") + "_" + id + ".jsonl"
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(log.String()), 0600); err != nil {
			t.Fatal(err)
		}
		mtime := day.AddDate(0, 0, offsets[len(offsets)-1]).Add(time.Hour)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	const active = "574f982b-5de5-4bae-b49f-94f0bc3250e4"
	history(active, -1, 0, 1)
	history("b7ed972c-e6bd-46e6-92d9-a797feb8d85b", -1, 1)
	history("287cf19c-0e72-4c76-8a52-677052e216c8", 1)

	output := t.TempDir()
	for range 2 {
		result, err := saveDay("cindy", "20261122", output)
		if err != nil || result.Matched != 1 || result.Exported != 1 {
			t.Fatalf("saveDay = %+v, %v", result, err)
		}
		if result.Path != filepath.Join(output, "ff-cindy-20261122.zip") {
			t.Fatalf("archive path = %q", result.Path)
		}
		checkCindyArchive(t, result.Path, active)
	}
	if result, err := saveDay("cindy", "20261124", output); err != nil || result.Path != "" || result.Matched != 0 {
		t.Fatalf("empty day = %+v, %v", result, err)
	}
	if _, err := saveDay("cindy", "20260230", output); err == nil {
		t.Fatal("invalid date was accepted")
	}
}

func checkCindyArchive(t *testing.T, path, id string) {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	read := func(name string) []byte {
		t.Helper()
		f, err := archive.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	var manifest struct {
		Date     string
		Sessions []archiveEntry
	}
	err = json.Unmarshal(read("manifest.json"), &manifest)
	if err != nil || manifest.Date != "2026-11-22" || len(manifest.Sessions) != 1 || manifest.Sessions[0].ID != id {
		t.Fatalf("manifest = %+v, %v", manifest, err)
	}
	markdown := string(read(manifest.Sessions[0].Markdown))
	for _, turn := range []string{"fixture day -1", "fixture day 0", "fixture day 1"} {
		if !strings.Contains(markdown, turn) {
			t.Fatalf("missing %q from Markdown", turn)
		}
	}
}
