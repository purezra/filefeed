package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo"
)

type AgentSession struct {
	Key       string   `json:"key"`
	Agent     string   `json:"agent"`
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Folder    string   `json:"folder"`
	Path      string   `json:"path"`
	Started   string   `json:"started"`
	Updated   string   `json:"updated"`
	RangeLast string   `json:"rangeLast,omitempty"`
	Models    []string `json:"models"`
	Size      int64    `json:"size"`
	Messages  int      `json:"messages"`
	Input     int      `json:"input"`
	Output    int      `json:"output"`
	Cache     int      `json:"cache"`
	Readable  bool     `json:"readable"`
	Channel   string   `json:"channel,omitempty"`
	TopicID   string   `json:"topicId,omitempty"`
	Topic     string   `json:"topic,omitempty"`
}

type AgentPart struct {
	Role string `json:"role"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	Text string `json:"text"`
	Cut  int    `json:"cut"`
}

type AgentTranscript struct {
	Parts  []AgentPart `json:"parts"`
	Before int         `json:"before"`
}

type AgentDateRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type AgentDateMatches struct {
	Rows    []AgentSession `json:"rows"`
	Skipped int            `json:"skipped"`
}

type AgentArchive struct {
	Path     string `json:"path"`
	Matched  int    `json:"matched"`
	Exported int    `json:"exported"`
}

type AgentExportItem struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Folder string `json:"folder"`
}

type AgentBatch struct {
	Path     string `json:"path"`
	Matched  int    `json:"matched"`
	Exported int    `json:"exported"`
	Skipped  int    `json:"skipped"`
	Empty    int    `json:"empty"`
}

func (Service) SessionCLI() (string, error) {
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return "", err
	}
	for _, p := range []string{
		filepath.Join(dir, "ff.exe"),
		filepath.Join(dir, runtime.GOOS+"-"+runtime.GOARCH, "ff.exe"),
	} {
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p, nil
		}
	}
	return "", errors.New("缺少会话读取组件 ff.exe，请使用完整的 FileFeed 安装包")
}

func (s Service) sessionCommand(ctx context.Context, result any, args ...string) error {
	path, err := s.SessionCLI()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, append([]string{"--filefeed"}, args...)...)
	hideWindow(cmd)
	data, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("会话操作已中止：%w", ctx.Err())
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) != 0 {
			return errors.New(strings.TrimSpace(string(exit.Stderr)))
		}
		return err
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(data, result)
}

func (s Service) AgentSessions(ctx context.Context) ([]AgentSession, error) {
	var rows []AgentSession
	err := s.sessionCommand(ctx, &rows, "list")
	return rows, err
}

func (s Service) AgentDateSessions(ctx context.Context, agent string, dates AgentDateRange) (AgentDateMatches, error) {
	var result AgentDateMatches
	err := s.sessionCommand(ctx, &result, "list-range", agent, dates.From, dates.To)
	return result, err
}

func (s Service) AgentTranscript(ctx context.Context, key string, clean bool, dates AgentDateRange, before int) (AgentTranscript, error) {
	var transcript AgentTranscript
	mode := "full"
	if clean {
		mode = "clean"
	}
	err := s.sessionCommand(ctx, &transcript, "read", key, mode, dates.From, dates.To, strconv.Itoa(before))
	return transcript, err
}

func sessionExportFilename(agent string, exportedAt time.Time, clean bool) string {
	agent = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, agent)
	if agent == "" {
		agent = "agent"
	}
	name := agent + "_" + exportedAt.Format("200601021504") + "_filefeed"
	if clean {
		name += "_clean"
	}
	return name + ".md"
}

func (s Service) ExportAgentSession(ctx context.Context, key, agent string, clean bool, dates AgentDateRange) (string, error) {
	command := "export"
	if clean {
		command = "export-clean"
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent: mygo.CallerWindow(ctx), Title: "导出会话 Markdown",
		DefaultPath: sessionExportFilename(agent, time.Now(), clean),
		Filters:     []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md"}}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := s.sessionCommand(ctx, nil, command, key, path, dates.From, dates.To); err != nil {
		return "", err
	}
	return path, nil
}

func (s Service) ExportAgentSessions(ctx context.Context, group string, items []AgentExportItem, clean bool, format string, dates AgentDateRange) (AgentBatch, error) {
	var result AgentBatch
	if len(items) == 0 {
		return result, errors.New("没有可导出的会话")
	}
	var output string
	switch format {
	case "zip", "markdown":
		extension, title, filter := "zip", "导出会话压缩包", "ZIP"
		if format == "markdown" {
			extension, title, filter = "md", "合并导出会话 Markdown", "Markdown"
		}
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent: mygo.CallerWindow(ctx), Title: title,
			DefaultPath: batchExportFilename(group, time.Now(), extension),
			Filters:     []mygo.FileFilter{{Name: filter, Extensions: []string{extension}}},
		})
		if err != nil || path == "" {
			return result, err
		}
		output = path
	default:
		return result, errors.New("无效的会话导出格式")
	}
	manifest, err := json.Marshal(struct {
		AgentDateRange
		Output  string            `json:"output"`
		Zip     bool              `json:"zip"`
		Merge   bool              `json:"merge"`
		Clean   bool              `json:"clean"`
		Name    string            `json:"name"`
		Entries []AgentExportItem `json:"entries"`
	}{dates, output, format == "zip", format == "markdown", clean, group, items})
	if err != nil {
		return result, err
	}
	file, err := os.CreateTemp("", "filefeed-batch-*.json")
	if err != nil {
		return result, err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(manifest); err != nil {
		file.Close()
		return result, err
	}
	if err := file.Close(); err != nil {
		return result, err
	}
	err = s.sessionCommand(ctx, &result, "batch", file.Name())
	return result, err
}

func batchExportFilename(group string, at time.Time, extension string) string {
	name := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ' ' {
			return r
		}
		return '-'
	}, group)
	name = strings.Join(strings.Fields(name), " ")
	runes := []rune(strings.Trim(name, " -"))
	if len(runes) > 50 {
		runes = runes[:50]
	}
	name = strings.TrimSpace(string(runes))
	if name == "" {
		name = "filefeed"
	}
	return name + "_" + at.Format("200601021504") + "." + extension
}

func (s Service) SaveAgentArchive(ctx context.Context, agent, date string) (AgentArchive, error) {
	var result AgentArchive
	err := s.sessionCommand(ctx, &result, "archive", agent, date)
	return result, err
}
