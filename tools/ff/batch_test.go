package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/sessions"
)

func TestSafeBatchName(t *testing.T) {
	cases := map[string]string{
		"存图专用":         "存图专用",
		"a/b:c?d*e":    "a-b-c-d-e",
		"  ..name..  ": "name",
		"a\n\tb":       "a b",
	}
	for in, want := range cases {
		if got := safeBatchName(in); got != want {
			t.Errorf("safeBatchName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBatchFilename(t *testing.T) {
	when := time.Date(2026, 11, 16, 12, 0, 0, 0, time.Local)
	s := sessions.Session{ID: "01a1e271-6acf", Title: "TG · 存图", Last: when}
	if got := batchFilename(4, batchEntry{Name: "存图/专用"}, s); got != "005-20261116-存图-专用-01a1e271.md" {
		t.Errorf("batchFilename = %q", got)
	}
	if got := batchFilename(0, batchEntry{}, s); got != "001-20261116-TG · 存图-01a1e271.md" {
		t.Errorf("title fallback = %q", got)
	}
	note := sessions.Session{ID: "01a1e390-b198", Last: when}
	entry := batchEntry{Name: "[渠道说明] 系统追加，不是用户消息。本条来自 Telegram 群「forcindy」", Folder: "日本影视作品"}
	if got := batchFilename(0, entry, note); got != "001-20261116-日本影视作品-01a1e390.md" {
		t.Errorf("topic fallback = %q", got)
	}
	if got := batchFilename(0, batchEntry{}, sessions.Session{ID: "abc"}); got != "001-abc.md" {
		t.Errorf("id fallback = %q", got)
	}
}

func TestZipEntryName(t *testing.T) {
	if got := zipEntryName("Telegram 群 · forcindy", "存图/专用", "001-a.md"); got != "Telegram 群 · forcindy/存图-专用/001-a.md" {
		t.Errorf("zipEntryName = %q", got)
	}
	if got := zipEntryName("", "", "001-a.md"); got != "filefeed/001-a.md" {
		t.Errorf("zipEntryName default = %q", got)
	}
}

func TestExportBatchRejectsEmptyManifest(t *testing.T) {
	dir := t.TempDir()
	data, err := json.Marshal(batchManifest{Output: filepath.Join(dir, "out")})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := exportBatch(path); err == nil || !strings.Contains(err.Error(), "没有可导出的会话") {
		t.Fatalf("err = %v", err)
	}
}
