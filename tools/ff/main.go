package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/appdir"
	"github.com/yetone/magpie/internal/sessions"
	"github.com/yetone/magpie/internal/settings"
)

const usage = `FileFeed 会话归档
  ff save <agent|all> <YYYYMMDD> [--output <目录>]
  ff ff-cindy-20261122 [--output <目录>]
默认保存到 %LOCALAPPDATA%\FileFeed\AgentArchives。`

var agentID = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Println(usage)
		return nil
	}
	if args[0] == "--filefeed" {
		return desktop(args[1:])
	}
	if shortcut := strings.TrimPrefix(args[0], "ff-"); shortcut != args[0] {
		if separator := strings.LastIndexByte(shortcut, '-'); separator > 0 {
			args = append([]string{"save", shortcut[:separator], shortcut[separator+1:]}, args[1:]...)
		}
	}
	if len(args) < 3 || args[0] != "save" {
		return errors.New(usage)
	}
	flags := flag.NewFlagSet("ff save", flag.ContinueOnError)
	output := flags.String("output", "", "归档目录")
	if err := flags.Parse(args[3:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New(usage)
	}
	result, err := saveDay(args[1], args[2], *output)
	if err != nil {
		return err
	}
	if result.Matched == 0 {
		fmt.Println("当天没有匹配的会话。")
		return nil
	}
	fmt.Printf("%s\n已保存 %d/%d 个会话的 Markdown。\n", result.Path, result.Exported, result.Matched)
	if result.Exported < result.Matched {
		return errors.New("部分会话只有元数据或读取失败，详情见归档内的 manifest.json。")
	}
	return nil
}

func appData() (string, error) {
	root, err := os.UserCacheDir()
	return filepath.Join(root, "FileFeed"), err
}

// loadTopicNames reads optional "<chat_id>#<topic_id>" names so topics whose
// name only exists in the Telegram UI can still be shown.
func loadTopicNames() map[string]string {
	names := map[string]string{}
	root, err := appData()
	if err != nil {
		return names
	}
	data, err := os.ReadFile(filepath.Join(root, "AgentTopics.json"))
	if err != nil {
		return names
	}
	_ = json.Unmarshal(data, &names)
	return names
}

func openEngine() error {
	root, err := appData()
	if err != nil {
		return err
	}
	engine := filepath.Join(root, "session-engine")
	if err := os.MkdirAll(filepath.Join(engine, "data"), 0700); err != nil {
		return err
	}
	// Keep upstream caches and settings under FileFeed while preserving the
	// actual agent locations, including the user's XDG settings and AppData.
	appdir.UseExecutable(filepath.Join(engine, "ff.exe"))
	if reads := settings.Shared().SessionReads; reads != nil && reads.All {
		return nil
	}
	return settings.SetSessionReads(func(*settings.SessionReads) *settings.SessionReads {
		return &settings.SessionReads{Asked: true, All: true}
	})
}

type sessionRow struct {
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

func row(s sessions.Managed, names map[string]string) sessionRow {
	models := make([]string, 0, len(s.Models))
	for _, model := range s.Models {
		models = append(models, model.Model)
	}
	when := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Local().Format(time.RFC3339)
	}
	out := sessionRow{
		Key: sessions.SessionRef(s.Session), Agent: rowAgent(s.Session), ID: s.ID, Title: s.Title, Folder: s.Cwd,
		Path: s.Path, Started: when(s.Start), Updated: when(s.Last), Models: models, Size: s.Size, Messages: s.Messages,
		Input: s.Input, Output: s.Output, Cache: s.CacheRead + s.CacheWrite, Readable: s.Transcript,
	}
	if out.Agent == "cindy" && s.Path != "" {
		label := cindyChannelFile(s.Cwd, s.Path, names)
		out.Channel, out.TopicID, out.Topic = label.Channel, label.TopicID, label.Topic
	}
	return out
}

func localSessions(agent string) []sessions.Managed {
	defer sessions.ShareListing()()
	var rows []sessions.Managed
	for _, source := range sessions.Agents() {
		if agent != "all" && source.Agent != agent && !(agent == "claude-desktop" && source.Agent == "claude") {
			continue
		}
		for _, s := range sessions.ListAgent(source.Agent) {
			if agent == "all" || agent == rowAgent(s.Session) {
				rows = append(rows, s)
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Last.After(rows[j].Last) })
	return rows
}

func desktop(args []string) error {
	if len(args) == 0 {
		return errors.New("缺少会话操作")
	}
	if args[0] == "archive" && len(args) == 3 {
		result, err := saveDay(args[1], args[2], "")
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	if err := openEngine(); err != nil {
		return err
	}
	defer sessions.Saved()
	defer sessions.OneListing()()
	encode := json.NewEncoder(os.Stdout).Encode
	switch {
	case args[0] == "list" && len(args) == 1:
		all := localSessions("all")
		rows := make([]sessionRow, 0, len(all))
		names := loadTopicNames()
		for _, s := range all {
			rows = append(rows, row(s, names))
		}
		return encode(rows)
	case args[0] == "batch" && len(args) == 2:
		result, err := exportBatch(args[1])
		if err != nil {
			return err
		}
		return encode(result)
	case (args[0] == "read" && len(args) == 2) || ((args[0] == "export" || args[0] == "export-clean") && len(args) == 3):
		agent, id, _ := strings.Cut(args[1], ":")
		s, ok := sessions.Find(agent, id)
		if !ok {
			return errors.New("找不到该会话，请刷新列表")
		}
		if len(args) == 3 {
			return exportMarkdown(s, args[2], args[0] == "export-clean")
		}
		transcript, err := sessions.TranscriptOf(s)
		if err != nil {
			return err
		}
		return encode(transcript)
	default:
		return errors.New("无效的会话操作")
	}
}
