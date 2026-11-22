package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/yetone/magpie/internal/sessions"
)

type batchEntry struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Folder string `json:"folder"`
}

type batchManifest struct {
	Output  string       `json:"output"`
	Zip     bool         `json:"zip"`
	Clean   bool         `json:"clean"`
	Name    string       `json:"name"`
	Entries []batchEntry `json:"entries"`
}

type batchResult struct {
	Path     string   `json:"path"`
	Matched  int      `json:"matched"`
	Exported int      `json:"exported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// exportBatch writes the sessions named in a manifest as Markdown files or as
// one ZIP. Each session is written independently, so a reader that only has
// metadata is reported and skipped instead of failing the whole batch.
func exportBatch(manifestPath string) (batchResult, error) {
	var result batchResult
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return result, err
	}
	var manifest batchManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return result, fmt.Errorf("无效的批量导出清单：%w", err)
	}
	if manifest.Output == "" {
		return result, fmt.Errorf("缺少导出位置")
	}
	if len(manifest.Entries) == 0 {
		return result, fmt.Errorf("没有可导出的会话")
	}
	result.Matched = len(manifest.Entries)
	if manifest.Zip {
		err = writeAtomic(manifest.Output, func(w io.Writer) error {
			archive := zip.NewWriter(w)
			for i, entry := range manifest.Entries {
				s, ok := findBatchSession(entry.Key)
				if !ok {
					result.skip(entry, "找不到该会话")
					continue
				}
				if !s.Transcript {
					result.skip(entry, "当前读取器仅支持该会话的元数据")
					continue
				}
				// Stage each export so a reader error can't leave a partial entry.
				var body bytes.Buffer
				if err := writeMarkdown(&body, s, manifest.Clean); err != nil {
					result.skip(entry, err.Error())
					continue
				}
				out, err := archive.Create(zipEntryName(manifest.Name, entry.Folder, batchFilename(i, entry, s)))
				if err != nil {
					return err
				}
				if _, err := out.Write(body.Bytes()); err != nil {
					return err
				}
				result.Exported++
			}
			return archive.Close()
		})
	} else {
		if err := os.MkdirAll(manifest.Output, 0o700); err != nil {
			return result, err
		}
		for i, entry := range manifest.Entries {
			s, ok := findBatchSession(entry.Key)
			if !ok {
				result.skip(entry, "找不到该会话")
				continue
			}
			if !s.Transcript {
				result.skip(entry, "当前读取器仅支持该会话的元数据")
				continue
			}
			dir := manifest.Output
			if folder := safeBatchName(entry.Folder); folder != "" {
				dir = filepath.Join(dir, folder)
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				result.skip(entry, err.Error())
				continue
			}
			if err := writeAtomic(filepath.Join(dir, batchFilename(i, entry, s)), func(w io.Writer) error {
				return writeMarkdown(w, s, manifest.Clean)
			}); err != nil {
				result.skip(entry, err.Error())
				continue
			}
			result.Exported++
		}
	}
	result.Path = manifest.Output
	return result, nil
}

func (r *batchResult) skip(entry batchEntry, reason string) {
	r.Skipped++
	name := entry.Name
	if name == "" {
		name = entry.Key
	}
	r.Errors = append(r.Errors, name+": "+reason)
}

func findBatchSession(key string) (sessions.Session, bool) {
	agent, id, ok := strings.Cut(key, ":")
	if !ok {
		return sessions.Session{}, false
	}
	return sessions.Find(agent, id)
}

func batchFilename(n int, entry batchEntry, s sessions.Session) string {
	name := batchStem(entry, s)
	prefix := fmt.Sprintf("%03d", n+1)
	if !s.Last.IsZero() {
		prefix += "-" + s.Last.Local().Format("20060102")
	}
	if name == "" || name == s.ID {
		return prefix + "-" + s.ID + ".md"
	}
	id := s.ID
	if len(id) > 8 {
		id = id[:8]
	}
	return prefix + "-" + name + "-" + id + ".md"
}

// batchStem prefers the name the caller gave, then the topic, then the
// session title. Cindy's channel note is a prompt prefix rather than a title,
// so it is not useful as a file name.
func batchStem(entry batchEntry, s sessions.Session) string {
	name := safeBatchName(entry.Name)
	if boilerplateBatchName(name) {
		name = safeBatchName(entry.Folder)
	}
	if name == "" {
		name = safeBatchName(s.Title)
	}
	if boilerplateBatchName(name) {
		name = ""
	}
	if name == "" {
		name = s.ID
	}
	if runes := []rune(name); len(runes) > 40 {
		name = string(runes[:40])
	}
	return name
}

func boilerplateBatchName(name string) bool {
	return strings.HasPrefix(name, "[渠道说明]") || strings.Contains(name, "本条来自 Telegram") || strings.Contains(name, "本条来自 Discord")
}

func zipEntryName(root, folder, file string) string {
	parts := []string{safeBatchName(root)}
	if parts[0] == "" {
		parts[0] = "filefeed"
	}
	if folder = safeBatchName(folder); folder != "" {
		parts = append(parts, folder)
	}
	return path.Join(append(parts, file)...)
}

// safeBatchName keeps a display name usable as one file or folder name.
func safeBatchName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 {
			return ' '
		}
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '-'
		}
		return r
	}, name)
	name = strings.Join(strings.Fields(name), " ")
	runes := []rune(strings.Trim(name, " .-"))
	if len(runes) > 60 {
		runes = runes[:60]
	}
	return strings.TrimSpace(string(runes))
}
