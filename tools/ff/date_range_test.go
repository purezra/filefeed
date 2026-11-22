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

	"github.com/yetone/magpie/internal/sessions"
)

func TestDateExportAcrossCindyTopicsAndPiFolders(t *testing.T) {
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
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	pi := filepath.Join(root, ".pi", "agent")
	t.Setenv("PI_CODING_AGENT_DIR", pi)
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", filepath.Join(root, "unused"))
	cindy := filepath.Join(config, "CindyGlobal", "pi-agent-home", "sessions")
	day := time.Date(2026, 11, 22, 0, 0, 0, 0, time.Local)
	makeSession := func(dir, id, prefix string, times ...time.Time) {
		t.Helper()
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		var log strings.Builder
		fmt.Fprintf(&log, `{"type":"session","version":3,"id":%q,"timestamp":%q,"cwd":%q}`+"\n", id, times[0].Format(time.RFC3339), dir)
		parent := ""
		for i, at := range times {
			key := fmt.Sprintf("m%d", i)
			fmt.Fprintf(&log, `{"type": "message","id":%q,"parentId":%q,"timestamp":%q,"message":{"role":"user","content":%q}}`+"\n",
				key, parent, at.UTC().Format(time.RFC3339Nano), fmt.Sprintf("%s-%d", prefix, i))
			parent = key
		}
		name := times[0].UTC().Format("2006-01-02T15-04-05-000Z") + "_" + id + ".jsonl"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(log.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	makeSession(cindy, "321b6fed-dc77-4c29-8192-5ce9da16a3c7", "topic-a", day.Add(-time.Second), day, day.AddDate(0, 0, 1))
	makeSession(cindy, "570a5c2c-f43f-4c4b-9fc6-7227efe138ac", "topic-b", day.AddDate(0, 0, -1), day.Add(time.Hour))
	folder := func(name string) string { return filepath.Join(pi, "sessions", name) }
	makeSession(folder("project-a"), "c68dd619-9195-4e56-9bec-6d64d8f61f65", "pi-a", day, day.AddDate(0, 0, 1))
	makeSession(folder("project-b"), "d7e88972-ef49-4863-944a-03cb359063ce", "pi-b", day.Add(time.Hour))
	makeSession(folder("project-c"), "d7dee295-72a0-4cc7-a8ad-d5ee3eec23dc", "pi-old", day.Add(-time.Second), day)

	if err := openEngine(); err != nil {
		t.Fatal(err)
	}
	defer sessions.OneListing()()
	dates, err := parseDateRange("2026-11-22", "2026-11-22")
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"cindy", "pi"} {
		result := listDateMatches(agent, dates)
		if result.Skipped != 0 || len(result.Rows) != 2 {
			t.Fatalf("%s matches = %+v", agent, result)
		}
		var entries []batchEntry
		for _, row := range result.Rows {
			entries = append(entries, batchEntry{Key: row.Key})
		}
		verify := func(markdown string) {
			t.Helper()
			want, absent := []string{"topic-a-1", "topic-b-1"}, []string{"topic-a-2"}
			if agent == "pi" {
				want, absent = []string{"pi-a-0", "pi-a-1", "pi-b-0"}, []string{"pi-old"}
			}
			// Session titles may quote the original first message.
			for _, text := range want {
				if !strings.Contains(markdown, "\n"+text+"\n") {
					t.Errorf("%s missing message %q", agent, text)
				}
			}
			for _, text := range append(absent, "topic-a-0", "topic-b-0") {
				if strings.Contains(markdown, "\n"+text+"\n") {
					t.Errorf("%s included out-of-range message %q", agent, text)
				}
			}
		}
		for _, merged := range []bool{false, true} {
			out := filepath.Join(root, agent+".zip")
			if merged {
				out = filepath.Join(root, agent+".md")
			}
			manifest := batchManifest{Output: out, Zip: !merged, Merge: merged, Clean: merged, Name: agent,
				From: "2026-11-22", To: "2026-11-22", Entries: entries}
			file := filepath.Join(root, "manifest.json")
			data, _ := json.Marshal(manifest)
			if err := os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := exportBatch(file); err != nil || got.Exported != 2 || got.Path != out {
				t.Fatalf("exportBatch = %+v, %v", got, err)
			}
			if merged {
				body, err := os.ReadFile(out)
				if err != nil {
					t.Fatal(err)
				}
				verify(string(body))
				continue
			}
			archive, err := zip.OpenReader(out)
			if err != nil {
				t.Fatal(err)
			}
			var body strings.Builder
			for _, file := range archive.File {
				reader, err := file.Open()
				if err != nil {
					t.Fatal(err)
				}
				_, err = io.Copy(&body, reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			archive.Close()
			verify(body.String())
		}
	}
}

func TestCleanPreviewStartsAtLatestAndPagesBack(t *testing.T) {
	f := filepath.Join(t.TempDir(), "long.jsonl")
	var log strings.Builder
	parent := ""
	for i := 1; i <= 205; i++ {
		id := fmt.Sprint(i)
		fmt.Fprintf(&log, `{"type":"message","id":%q,"parentId":%q,"message":{"role":"assistant","content":[{"type":"thinking","thinking":%q},{"type":"text","text":%q}]}}`+"\n",
			id, parent, strings.Repeat("a", 2500), id)
		parent = id
	}
	if err := os.WriteFile(f, []byte(log.String()), 0600); err != nil {
		t.Fatal(err)
	}
	s := sessions.Session{Agent: "cindy", Path: f}
	for _, test := range []struct {
		cursor, before int
		first, last    string
	}{{0, 105, "106", "205"}, {105, 5, "6", "105"}, {5, 0, "1", "5"}} {
		page, err := readPreview(s, true, dateRange{}, test.cursor)
		if err != nil || page.Before != test.before || len(page.Parts) == 0 ||
			page.Parts[0].Text != test.first || page.Parts[len(page.Parts)-1].Text != test.last {
			t.Fatalf("readPreview before=%d returned %d parts / cursor %d, %v", test.cursor, len(page.Parts), page.Before, err)
		}
	}
}

func TestDateRangeAllowsEitherBound(t *testing.T) {
	day := time.Date(2026, 11, 22, 0, 0, 0, 0, time.Local)
	since, err := parseDateRange("2026-11-22", "")
	if err != nil || !since.includes(day) || since.includes(day.Add(-time.Nanosecond)) {
		t.Fatalf("invalid lower bound: %v, %v", since, err)
	}
	until, err := parseDateRange("", "2026-11-22")
	if err != nil || !until.includes(day.AddDate(0, 0, 1).Add(-time.Nanosecond)) || until.includes(day.AddDate(0, 0, 1)) {
		t.Fatalf("invalid upper bound: %v, %v", until, err)
	}
	if _, err := parseDateRange("2026-11-23", "2026-11-22"); err == nil {
		t.Fatal("reversed dates accepted")
	}
}
