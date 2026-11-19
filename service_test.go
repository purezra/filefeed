package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRefreshConfig(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := os.WriteFile(configPath(), []byte(`{"exts":["md"],"limit":50}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Service{}
	cfg := s.Config()
	if cfg.RefreshSeconds != 60 || cfg.Limit != 50 {
		t.Fatalf("unexpected legacy config: %+v", cfg)
	}
	for _, interval := range []int{5, 10, 15, 30, 60} {
		cfg.RefreshSeconds = interval
		s.SaveConfig(cfg)
		if got := s.Config().RefreshSeconds; got != interval {
			t.Fatalf("refresh interval after saving %d = %d", interval, got)
		}
	}
	cfg.RefreshSeconds = 1
	if got := s.SaveConfig(cfg).RefreshSeconds; got != 60 {
		t.Fatalf("invalid refresh interval was saved as %d", got)
	}
}

func TestQuickTypesConfig(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	s := Service{}
	cfg := s.Config()
	if len(cfg.QuickTypes) != 0 {
		t.Fatal("legacy config should use the built-in shortcuts")
	}
	cfg.QuickTypes["video"] = []string{"mp4", "webm"}
	cfg.QuickTypes["audio"] = []string{}
	s.SaveConfig(cfg)
	loaded := s.Config().QuickTypes
	if video := loaded["video"]; len(video) != 2 || video[0] != "mp4" || video[1] != "webm" {
		t.Fatalf("wrong saved video shortcut: %v", video)
	}
	if audio, exists := loaded["audio"]; !exists || len(audio) != 0 {
		t.Fatalf("an unchecked category must stay empty: %v %v", audio, exists)
	}
}

func TestSearch(t *testing.T) {
	s := Service{}
	res := s.Search(SearchQuery{
		Exts: []string{"md"}, Mode: "past", PastValue: 24, PastUnit: "hour",
		TimeField: "created", Limit: 2, SortBy: "time",
	})
	if !res.OK {
		t.Fatalf("search failed: %s", res.Error)
	}
	b, _ := json.Marshal(res.Groups)
	if len(b) > 400 {
		b = b[:400]
	}
	t.Logf("total=%d elapsed=%dms %s", res.Total, res.ElapsedMs, string(b))
}

func TestMultiCopy(t *testing.T) {
	s := Service{}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		filepath.Join(dir, "README.md"),
		filepath.Join(dir, "main.go"),
	}
	if err := s.CopyFiles(paths); err != "" {
		t.Fatalf("CopyFiles: %s", err)
	}
	t.Log("多文件复制 OK")
}
