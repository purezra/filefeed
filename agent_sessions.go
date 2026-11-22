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
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo"
)

type AgentSession struct {
	Key      string   `json:"key"`
	Agent    string   `json:"agent"`
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Folder   string   `json:"folder"`
	Path     string   `json:"path"`
	Started  string   `json:"started"`
	Updated  string   `json:"updated"`
	Models   []string `json:"models"`
	Size     int64    `json:"size"`
	Messages int      `json:"messages"`
	Input    int      `json:"input"`
	Output   int      `json:"output"`
	Cache    int      `json:"cache"`
	Readable bool     `json:"readable"`
	Channel  string   `json:"channel,omitempty"`
	TopicID  string   `json:"topicId,omitempty"`
	Topic    string   `json:"topic,omitempty"`
}

type AgentPart struct {
	Role string `json:"role"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	Text string `json:"text"`
	Cut  int    `json:"cut"`
}

type AgentTranscript struct {
	Parts []AgentPart `json:"parts"`
	Cut   bool        `json:"cut"`
}

type AgentArchive struct {
	Path     string `json:"path"`
	Matched  int    `json:"matched"`
	Exported int    `json:"exported"`
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

func (s Service) AgentTranscript(ctx context.Context, key string) (AgentTranscript, error) {
	var transcript AgentTranscript
	err := s.sessionCommand(ctx, &transcript, "read", key)
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

func (s Service) ExportAgentSession(ctx context.Context, key, agent string, clean bool) (string, error) {
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
	if err := s.sessionCommand(ctx, nil, command, key, path); err != nil {
		return "", err
	}
	return path, nil
}

func (s Service) SaveAgentArchive(ctx context.Context, agent, date string) (AgentArchive, error) {
	var result AgentArchive
	err := s.sessionCommand(ctx, &result, "archive", agent, date)
	return result, err
}
