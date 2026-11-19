package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------- 类型定义

// SearchQuery 是一次检索的全部条件，前端整包传过来。
type SearchQuery struct {
	Exts      []string `json:"exts"`      // 扩展名（不带点），空 = 不限（会很慢）
	Keyword   string   `json:"keyword"`   // 文件名/路径包含的关键词
	Mode      string   `json:"mode"`      // "past" 过去一段时间 | "range" 自定义区间
	PastValue int      `json:"pastValue"` // 配合 PastUnit
	PastUnit  string   `json:"pastUnit"`  // minute | hour | day | week
	From      string   `json:"from"`      // "2006-01-02T15:04" 本地时间
	To        string   `json:"to"`
	TimeField string   `json:"timeField"` // "created" | "modified"
	Exclude   []string `json:"exclude"`   // 路径包含这些片段就跳过（不区分大小写）
	Limit     int      `json:"limit"`     // 每个类型最多返回多少条
	SortBy    string   `json:"sortBy"`    // "time" | "size" | "name"
}

// FileItem 是结果里的一行。
type FileItem struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Dir        string `json:"dir"`
	Ext        string `json:"ext"`
	Size       int64  `json:"size"`
	SizeText   string `json:"sizeText"`
	CreatedAt  int64  `json:"createdAt"` // 秒
	ModifiedAt int64  `json:"modifiedAt"`
	Stamp      int64  `json:"stamp"` // 参与排序/过滤的那个时间
	IsNew      bool   `json:"isNew"`
}

// TypeGroup 是按扩展名分好的一组结果。
type TypeGroup struct {
	Type  string     `json:"type"`
	Count int        `json:"count"` // 命中总数（可能大于 Items 长度）
	Items []FileItem `json:"items"`
}

// SearchResult 是给前端的完整响应。
type SearchResult struct {
	OK        bool        `json:"ok"`
	Error     string      `json:"error"`
	Groups    []TypeGroup `json:"groups"`
	Total     int         `json:"total"`
	ElapsedMs int64       `json:"elapsedMs"`
	Lo        int64       `json:"lo"`
	Hi        int64       `json:"hi"`
}

// Preset 是保存下来的筛选方案。
type Preset struct {
	Name  string      `json:"name"`
	Query SearchQuery `json:"query"`
}

// Config 是持久化配置。
type Config struct {
	Exts           []string `json:"exts"`
	Exclude        []string `json:"exclude"`
	Limit          int      `json:"limit"`
	RefreshSeconds int      `json:"refreshSeconds"`
	Presets        []Preset `json:"presets"`
}

// Service 的方法会被 mygo generate 变成前端的类型化函数。
type Service struct{}

// ---------------------------------------------------------------- 配置

const defaultRefreshSeconds = 60

func validRefreshSeconds(seconds int) int {
	switch seconds {
	case 5, 10, 15, 30, 60:
		return seconds
	default:
		return defaultRefreshSeconds
	}
}

func configPath() string {
	return filepath.Join(dataDir(), "config.json")
}

func defaultConfig() Config {
	return Config{
		Exts: []string{"md", "txt", "json", "jsonl", "csv", "xlsx", "doc", "docx", "pdf", "zip"},
		Exclude: []string{
			"\\node_modules\\", "\\$recycle.bin\\", "\\windows\\",
			"\\.git\\", "\\appdata\\local\\temp\\",
		},
		Limit:          500,
		RefreshSeconds: defaultRefreshSeconds,
		Presets:        []Preset{},
	}
}

// Config 返回当前配置（文件不存在时返回默认值）。
func (Service) Config() Config {
	cfg := defaultConfig()
	b, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	var user Config
	if json.Unmarshal(b, &user) != nil {
		return cfg
	}
	if len(user.Exts) > 0 {
		cfg.Exts = user.Exts
	}
	if len(user.Exclude) > 0 {
		cfg.Exclude = user.Exclude
	}
	if user.Limit > 0 {
		cfg.Limit = user.Limit
	}
	cfg.RefreshSeconds = validRefreshSeconds(user.RefreshSeconds)
	if user.Presets != nil {
		cfg.Presets = user.Presets
	}
	return cfg
}

// SaveConfig 写回配置。
func (Service) SaveConfig(cfg Config) Config {
	if len(cfg.Exts) == 0 {
		cfg.Exts = defaultConfig().Exts
	}
	if cfg.Limit <= 0 {
		cfg.Limit = 500
	}
	cfg.RefreshSeconds = validRefreshSeconds(cfg.RefreshSeconds)
	b, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(configPath(), b, 0o644)
	return cfg
}

// ---------------------------------------------------------------- 检索

func parseLocal(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func unitDuration(unit string) time.Duration {
	switch unit {
	case "minute":
		return time.Minute
	case "hour":
		return time.Hour
	case "day":
		return 24 * time.Hour
	case "week":
		return 7 * 24 * time.Hour
	}
	return time.Hour
}

func fmtSize(n int64) string {
	const k = 1024
	if n < k {
		return strconv.FormatInt(n, 10) + " B"
	}
	units := []string{"KB", "MB", "GB", "TB"}
	f := float64(n)
	for _, u := range units {
		f /= k
		if f < k {
			return strconv.FormatFloat(f, 'f', 1, 64) + " " + u
		}
	}
	return strconv.FormatFloat(f, 'f', 1, 64) + " PB"
}

// Search 执行一次检索。
func (Service) Search(q SearchQuery) SearchResult {
	start := time.Now()
	res := SearchResult{OK: true}

	// 时间窗口
	now := time.Now()
	var lo, hi time.Time
	switch q.Mode {
	case "range":
		lo = parseLocal(q.From)
		hi = parseLocal(q.To)
		if lo.IsZero() && hi.IsZero() {
			lo = now.Add(-24 * time.Hour)
		}
	default: // past
		v := q.PastValue
		if v <= 0 {
			v = 1
		}
		lo = now.Add(-time.Duration(v) * unitDuration(q.PastUnit))
	}
	res.Lo, res.Hi = lo.Unix(), 0
	if !hi.IsZero() {
		res.Hi = hi.Unix()
	}

	// 交给 Everything 的日期粗筛：走 USN 索引，能把 60 万候选压到几千条。
	// rc = 最近变更（含新建），是「新增文件」最贴切的索引字段。
	// 超出 USN 日志保留期时退回 dm（修改时间，全量已索引）。
	var prefilter []string
	if !lo.IsZero() {
		if time.Since(lo) <= 7*24*time.Hour {
			prefilter = append(prefilter, "rc:>="+lo.Format("2006-01-02"))
			if !hi.IsZero() {
				prefilter = append(prefilter, "rc:<="+hi.Format("2006-01-02"))
			}
		} else {
			prefilter = append(prefilter, "dm:>="+lo.Format("2006-01-02"))
			if !hi.IsZero() {
				prefilter = append(prefilter, "dm:<="+hi.Format("2006-01-02"))
			}
		}
	}

	paths, err := queryPaths(q.Exts, q.Keyword, prefilter)
	if err != nil {
		res.OK = false
		res.Error = "调用 es.exe 失败：" + err.Error()
		res.ElapsedMs = time.Since(start).Milliseconds()
		return res
	}

	excl := make([]string, 0, len(q.Exclude))
	for _, e := range q.Exclude {
		if s := strings.ToLower(strings.TrimSpace(e)); s != "" {
			excl = append(excl, s)
		}
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 500
	}

	buckets := map[string][]FileItem{}
	counts := map[string]int{}

	for _, p := range paths {
		low := strings.ToLower(p)
		skip := false
		for _, e := range excl {
			if strings.Contains(low, e) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		st, serr := os.Stat(p)
		if serr != nil || st.IsDir() {
			continue
		}

		created := st.ModTime()
		modified := st.ModTime()
		if c, ok := creationTime(st); ok {
			created = c
		}

		stamp := modified
		if q.TimeField == "created" {
			stamp = created
		}
		if stamp.Before(lo) {
			continue
		}
		if !hi.IsZero() && stamp.After(hi) {
			continue
		}

		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(p), "."))
		if ext == "" {
			ext = "(无扩展名)"
		}

		item := FileItem{
			Path:       p,
			Name:       filepath.Base(p),
			Dir:        filepath.Dir(p),
			Ext:        ext,
			Size:       st.Size(),
			SizeText:   fmtSize(st.Size()),
			CreatedAt:  created.Unix(),
			ModifiedAt: modified.Unix(),
			Stamp:      stamp.Unix(),
			IsNew:      !created.Before(lo) && (hi.IsZero() || !created.After(hi)),
		}
		counts[ext]++
		if len(buckets[ext]) < limit {
			buckets[ext] = append(buckets[ext], item)
		}
	}

	types := make([]string, 0, len(buckets))
	for t := range buckets {
		types = append(types, t)
	}
	// 类型按命中数从多到少
	sort.Slice(types, func(i, j int) bool {
		if counts[types[i]] != counts[types[j]] {
			return counts[types[i]] > counts[types[j]]
		}
		return types[i] < types[j]
	})

	for _, t := range types {
		items := buckets[t]
		switch q.SortBy {
		case "size":
			sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
		case "name":
			sort.Slice(items, func(i, j int) bool {
				return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
			})
		default: // time
			sort.Slice(items, func(i, j int) bool { return items[i].Stamp > items[j].Stamp })
		}
		res.Groups = append(res.Groups, TypeGroup{Type: t, Count: counts[t], Items: items})
		res.Total += counts[t]
	}

	res.ElapsedMs = time.Since(start).Milliseconds()
	return res
}

// ---------------------------------------------------------------- 文件操作

// Open 用系统默认程序打开文件，返回错误信息（空串表示成功）。
// 注意：不能给 GUI 子进程设 HideWindow，否则打开的程序窗口会是隐藏的。
func (Service) Open(path string) string {
	if _, err := os.Stat(path); err != nil {
		return "文件不存在"
	}
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	if err := cmd.Start(); err != nil {
		return err.Error()
	}
	return ""
}

// Reveal 在资源管理器里选中该文件。
func (Service) Reveal(path string) string {
	if _, err := os.Stat(path); err != nil {
		return "文件不存在"
	}
	// 同上：explorer 的窗口不能被 HideWindow 影响
	cmd := exec.Command("explorer", "/select,"+path)
	if err := cmd.Start(); err != nil {
		return err.Error()
	}
	return ""
}

// CopyFiles 把文件本身放到剪贴板（CF_HDROP），支持多选，
// 之后可以在资源管理器里直接 Ctrl+V 粘贴出文件。
func (Service) CopyFiles(paths []string) string {
	ok := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			ok = append(ok, p)
		}
	}
	if len(ok) == 0 {
		return "没有可复制的文件"
	}
	if err := copyFilesToClipboard(ok); err != nil {
		return err.Error()
	}
	return ""
}

// DeleteFiles 把多个文件一起移到回收站。
// 路径写进临时文件再由 PowerShell 读，避免命令行长度限制和转义问题。
func (Service) DeleteFiles(paths []string) string {
	exist := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			exist = append(exist, p)
		}
	}
	if len(exist) == 0 {
		return "没有可删除的文件"
	}

	listPath := filepath.Join(os.TempDir(), "filefeed-delete.txt")
	if err := os.WriteFile(listPath, []byte(strings.Join(exist, "\n")), 0o644); err != nil {
		return err.Error()
	}
	defer os.Remove(listPath)

	script := "Add-Type -AssemblyName Microsoft.VisualBasic; " +
		"Get-Content -LiteralPath '" + listPath + "' -Encoding UTF8 | " +
		"Where-Object { $_ -ne '' } | ForEach-Object { " +
		"try { [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile($_, 'OnlyErrorDialogs', 'SendToRecycleBin') } catch {} }"

	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return msg
	}
	return ""
}
