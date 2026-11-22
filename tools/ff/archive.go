package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/sessions"
)

type archiveResult struct {
	Path     string `json:"path"`
	Matched  int    `json:"matched"`
	Exported int    `json:"exported"`
}

type archiveEntry struct {
	sessionRow
	Markdown string `json:"markdown,omitempty"`
	Error    string `json:"error,omitempty"`
}

func saveDay(agent, date, output string) (archiveResult, error) {
	var result archiveResult
	agent = strings.ToLower(agent)
	if !agentID.MatchString(agent) {
		return result, fmt.Errorf("无效的 Agent ID %q", agent)
	}
	day, err := time.ParseInLocation("20060102", date, time.Local)
	if err != nil || len(date) != 8 || day.Year() < 1 {
		return result, fmt.Errorf("无效日期 %q，请使用 YYYYMMDD", date)
	}
	if err := openEngine(); err != nil {
		return result, err
	}
	defer sessions.Saved()
	defer sessions.OneListing()()
	all := localSessions(agent)
	if len(all) == 0 && agent != "all" {
		return result, fmt.Errorf("未发现 Agent %q 的本地会话", agent)
	}
	active := make(map[string]bool)
	for _, key := range sessions.StatsAt(1, day).Keys {
		active[key] = true
	}
	var selected []sessions.Managed
	end := day.AddDate(0, 0, 1)
	inDay := func(t time.Time) bool { return !t.Before(day) && t.Before(end) }
	for _, s := range all {
		if active[sessions.SessionRef(s.Session)] || inDay(s.Start) || inDay(s.Last) {
			selected = append(selected, s)
		}
	}
	result.Matched = len(selected)
	if result.Matched == 0 {
		return result, nil
	}
	if output == "" {
		root, err := appData()
		if err != nil {
			return result, err
		}
		output = filepath.Join(root, "AgentArchives")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return result, err
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		return result, err
	}
	path := filepath.Join(output, fmt.Sprintf("ff-%s-%s.zip", agent, date))
	err = writeAtomic(path, func(w io.Writer) error {
		archive := zip.NewWriter(w)
		entries := make([]archiveEntry, 0, len(selected))
		names := loadTopicNames()
		for i, s := range selected {
			entry := archiveEntry{sessionRow: row(s, names)}
			if !entry.Readable {
				entry.Error = "当前读取器仅支持该会话的元数据"
			} else if err := addMarkdown(archive, output, i+1, s.Session, &entry); err != nil {
				return err
			}
			if entry.Markdown != "" {
				result.Exported++
			}
			entries = append(entries, entry)
		}
		manifest, err := archive.Create("manifest.json")
		if err != nil {
			return err
		}
		enc := json.NewEncoder(manifest)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Date     string         `json:"date"`
			TimeZone string         `json:"timeZone"`
			Agent    string         `json:"agent"`
			Sessions []archiveEntry `json:"sessions"`
		}{day.Format(time.DateOnly), day.Format("-07:00"), agent, entries}); err != nil {
			return err
		}
		return archive.Close()
	})
	if err != nil {
		return archiveResult{}, err
	}
	result.Path = path
	return result, nil
}

func rowAgent(s sessions.Session) string {
	if s.App != "" {
		return s.App
	}
	return s.Agent
}

func addMarkdown(archive *zip.Writer, dir string, n int, s sessions.Session, entry *archiveEntry) error {
	// Stage each export so a reader error can't leave a partial ZIP entry.
	f, err := os.CreateTemp(dir, ".ff-markdown-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := sessions.WriteMarkdown(f, s, entry.Agent); err != nil {
		entry.Error = err.Error()
		return nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	name := fmt.Sprintf("%04d-%s.md", n, entry.Agent)
	w, err := archive.Create(name)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	entry.Markdown = name
	return nil
}

func writeMarkdown(w io.Writer, s sessions.Session, clean bool, dates dateRange) error {
	if !dates.active() || s.Agent != "cindy" {
		if dates.active() && !dates.includes(s.Start) {
			return errNoDatedContent
		}
		if clean {
			return writeCleanMarkdown(w, s, func(each func(sessions.Part) bool) error {
				return sessions.FileFeedParts(s, each)
			})
		}
		return sessions.WriteMarkdown(w, s, rowAgent(s))
	}
	var parts []sessions.Part
	hasContent := false
	s.Start, s.Last = time.Time{}, time.Time{}
	if err := dates.parts(s, func(at time.Time, part sessions.Part) bool {
		if clean && part.Role == "user" && part.Kind == "text" {
			part.Text = cleanDatedCindyQuestion(part.Text)
		}
		parts = append(parts, part)
		if clean && cleanMarkdownText(part) == "" {
			return true
		}
		hasContent = true
		if s.Start.IsZero() || at.Before(s.Start) {
			s.Start = at
		}
		if at.After(s.Last) {
			s.Last = at
		}
		return true
	}); err != nil {
		return err
	}
	if !hasContent {
		return errNoDatedContent
	}
	read := func(each func(sessions.Part) bool) error {
		for _, part := range parts {
			if !each(part) {
				break
			}
		}
		return nil
	}
	if clean {
		return writeCleanMarkdown(w, s, read)
	}
	return sessions.FileFeedWriteMarkdown(w, s, rowAgent(s), read)
}

func exportMarkdown(s sessions.Session, path string, clean bool, dates dateRange) error {
	return writeAtomic(path, func(w io.Writer) error {
		return writeMarkdown(w, s, clean, dates)
	})
}

func writeAtomic(path string, write func(io.Writer) error) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".ff-export-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = write(f)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
