package main

import (
	"bytes"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// es.exe 是 Everything 的官方命令行工具，直接嵌进二进制，
// 首次运行时释放到 %LOCALAPPDATA%\FileFeed，做到真正的单文件分发。
//
//go:embed assets/es.exe
var esBin []byte

var (
	esOnce sync.Once
	esPath string
	esErr  error
)

func dataDir() string {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "FileFeed")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func ensureEs() (string, error) {
	esOnce.Do(func() {
		p := filepath.Join(dataDir(), "es.exe")
		if st, err := os.Stat(p); err == nil && st.Size() == int64(len(esBin)) {
			esPath = p
			return
		}
		if err := os.WriteFile(p, esBin, 0o755); err != nil {
			esErr = err
			return
		}
		esPath = p
	})
	return esPath, esErr
}

// queryPaths 调用 es.exe 拿候选文件路径。
// exts 为空表示不限扩展名（会很慢，前端会拦）。
// dateFilter 是交给 Everything 的日期粗筛（如 "rc:>=2026-11-17"），
// 它走 USN 索引，能把 60 万候选压到几千条，是性能的关键。
func queryPaths(exts []string, keyword string, dateFilter []string) ([]string, error) {
	es, err := ensureEs()
	if err != nil {
		return nil, err
	}

	args := make([]string, 0, 6)
	if len(exts) > 0 {
		clean := make([]string, 0, len(exts))
		for _, e := range exts {
			e = strings.TrimSpace(strings.TrimPrefix(e, "."))
			if e != "" {
				clean = append(clean, e)
			}
		}
		if len(clean) > 0 {
			args = append(args, "ext:"+strings.Join(clean, ";"))
		}
	}
	args = append(args, dateFilter...)
	if k := strings.TrimSpace(keyword); k != "" {
		args = append(args, k)
	}

	cmd := exec.Command(es, args...)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		// es.exe 没匹配时返回码可能非 0，但 stdout 为空，按空结果处理
		if len(out) == 0 {
			return nil, nil
		}
		return nil, err
	}

	// es.exe 输出跟随系统代码页（中文 Windows 是 GBK）
	if decoded, derr := simplifiedchinese.GBK.NewDecoder().Bytes(out); derr == nil {
		out = decoded
	}

	var paths []string
	for _, line := range bytes.Split(out, []byte("\n")) {
		s := strings.TrimSpace(string(line))
		s = strings.TrimSuffix(s, "\r")
		if s != "" {
			paths = append(paths, s)
		}
	}
	return paths, nil
}
