package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// cindyLabel is what a Cindy session says about where it came from.
type cindyLabel struct {
	Channel string
	TopicID string
	Topic   string
}

// cindyChannelFile labels a Cindy session from its channel note and the
// conversation structure the session recorded.
func cindyChannelFile(folder, path string, names map[string]string) cindyLabel {
	body, err := os.ReadFile(path)
	if err != nil {
		body = nil
	}
	return cindyChannel(folder, body, path, names)
}

func cindyChannel(folder string, body []byte, path string, names map[string]string) cindyLabel {
	kind, name, chat, bot := scanCindyChannel(body)
	switch kind {
	case "tg-dm":
		who := bot
		if who == "" {
			who = strings.TrimPrefix(chat, "-")
		}
		if who == "" {
			who = "私聊"
		}
		return cindyLabel{Channel: "Telegram Bot · " + who}
	case "tg-group":
		channel := "Telegram 群 · " + name
		if name == "" {
			channel = "Telegram 群"
		}
		topicID, topic := groupTopic(body, chat, cindyTopicIDs(path, cindyBotID(folder, bot), chat), names)
		return cindyLabel{Channel: channel, TopicID: topicID, Topic: topic}
	case "dc-dm":
		return cindyLabel{Channel: "Discord 私聊"}
	case "dc-group":
		if name == "" {
			return cindyLabel{Channel: "Discord"}
		}
		return cindyLabel{Channel: "Discord · " + name}
	default:
		base := folderBase(folder)
		if rest, ok := strings.CutPrefix(base, "telegram-"); ok && rest != "" {
			return cindyLabel{Channel: "Telegram Bot · " + rest}
		}
		if strings.HasPrefix(base, "discord-") {
			return cindyLabel{Channel: "Discord · " + base}
		}
		return cindyLabel{Channel: "电脑端"}
	}
}

// groupTopic is the topic a Telegram group session belongs to. Only structure
// counts: the thread id the session actually used, or the root message the
// first turn replied to, which Telegram makes equal to the thread id. A
// passing mention of 话题「…」 in the conversation is not a topic name, so it
// is ignored.
func groupTopic(body []byte, chat string, valid map[string]bool, names map[string]string) (id, topic string) {
	if names == nil {
		names = map[string]string{}
	}
	topics := forumTopics(body)
	for thread, name := range topics {
		if name != "" && names[chat+"#"+thread] == "" {
			names[chat+"#"+thread] = name
		}
	}
	threads, order := countThreads(body)
	if valid == nil {
		valid = map[string]bool{}
	}
	for thread := range threads {
		valid[thread] = true
	}
	if thread := firstReplyTopic(body, valid); thread != "" {
		id = thread
	} else if thread, n := mostCounted(threads, order); n > 0 {
		id = thread
	}
	if id == "" {
		return "", ""
	}
	name := names[chat+"#"+id]
	if name == "" {
		name = topics[id]
	}
	if name != "" {
		if names[chat+"#"+id] == "" {
			names[chat+"#"+id] = name
		}
		return id, name
	}
	return id, "话题 " + id
}

// firstReplyTopic reads the topic id from the root message the first turn
// replied to. Telegram numbers a topic's root message with its thread id.
func firstReplyTopic(body []byte, valid map[string]bool) string {
	i := bytes.Index(body, []byte("<reply_context>"))
	if i < 0 {
		return ""
	}
	window := body[i:]
	if len(window) > 240 {
		window = window[:240]
	}
	hash := bytes.IndexByte(window, '#')
	if hash < 0 {
		return ""
	}
	start := hash + 1
	end := start
	for end < len(window) && window[end] >= '0' && window[end] <= '9' {
		end++
	}
	if end == start {
		return ""
	}
	id := string(window[start:end])
	if valid[id] {
		return id
	}
	return ""
}

// cindyBotID is the id in the session folder (telegram-9085529043). The note
// records the bot's display name, which is not what Cindy names its runtime
// files with.
func cindyBotID(folder, bot string) string {
	if rest, ok := strings.CutPrefix(folderBase(folder), "telegram-"); ok && rest != "" {
		return rest
	}
	return bot
}

// cindyTopicIDs lists the topics Cindy keeps per conversation next to the
// session files. Only ids are recorded there; names come from the sessions
// themselves.
func cindyTopicIDs(sessionPath, bot, chat string) map[string]bool {
	if sessionPath == "" || bot == "" || chat == "" {
		return nil
	}
	runtime := filepath.Join(filepath.Dir(filepath.Dir(sessionPath)), "runtime")
	entries, err := os.ReadDir(runtime)
	if err != nil {
		return nil
	}
	prefix := "telegram_" + bot + "_g-" + chat + "-"
	ids := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		at := strings.Index(name, prefix)
		if at < 0 {
			continue
		}
		rest := name[at+len(prefix):]
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		if end > 0 {
			ids[rest[:end]] = true
		}
	}
	return ids
}

func scanCindyChannel(body []byte) (kind, name, chat, bot string) {
	if i := bytes.Index(body, []byte("本条来自 ")); i >= 0 {
		rest := body[i+len("本条来自 "):]
		switch {
		case bytes.HasPrefix(rest, []byte("Telegram 群「")):
			kind, name = "tg-group", until(rest[len("Telegram 群「"):], "」")
		case bytes.HasPrefix(rest, []byte("Telegram 私聊")):
			kind = "tg-dm"
		case bytes.HasPrefix(rest, []byte("Discord 群「")):
			kind, name = "dc-group", until(rest[len("Discord 群「"):], "」")
		case bytes.HasPrefix(rest, []byte("Discord 私聊")):
			kind = "dc-dm"
		}
		if k := bytes.Index(rest, []byte("chat_id:")); k >= 0 && k < 160 {
			chat = leadingID(rest[k+len("chat_id:"):])
		}
	}
	bot = firstToken(body, [][]byte{[]byte("你的名字:"), []byte("你的名字：")})
	return kind, name, chat, bot
}

func until(b []byte, end string) string {
	j := bytes.Index(b, []byte(end))
	if j <= 0 || j > 80 {
		return ""
	}
	return strings.TrimSpace(string(b[:j]))
}

func leadingID(b []byte) string {
	b = bytes.TrimLeft(b, " \t")
	i := 0
	if i < len(b) && b[i] == '-' {
		i++
	}
	digits := i
	for digits < len(b) && b[digits] >= '0' && b[digits] <= '9' {
		digits++
	}
	if digits == i {
		return ""
	}
	return string(b[:digits])
}

func firstToken(b []byte, marks [][]byte) string {
	at, mark := -1, []byte(nil)
	for _, item := range marks {
		i := bytes.Index(b, item)
		if i >= 0 && (at < 0 || i < at) {
			at, mark = i, item
		}
	}
	if at < 0 {
		return ""
	}
	rest := bytes.TrimLeft(b[at+len(mark):], " \t")
	n := 0
	for n < len(rest) && n < 40 && rest[n] != ' ' && rest[n] != '\n' && rest[n] != '\r' && rest[n] != '\\' && rest[n] != '<' && rest[n] != '|' {
		n++
	}
	return string(rest[:n])
}

func countThreads(b []byte) (map[string]int, []string) {
	counts := map[string]int{}
	var order []string
	key := []byte("message_thread_id")
	for from := 0; ; {
		i := bytes.Index(b[from:], key)
		if i < 0 {
			return counts, order
		}
		i += from
		id := threadAt(b[i+len(key):])
		if id != "" {
			if counts[id] == 0 {
				order = append(order, id)
			}
			counts[id]++
		}
		from = i + len(key)
	}
}

func threadAt(b []byte) string {
	i := 0
	for i < len(b) && i < 12 {
		switch b[i] {
		case '\\', '"', '\'', ' ', '\t', ':':
			i++
		default:
			goto num
		}
	}
num:
	if i >= len(b) || b[i] < '0' || b[i] > '9' {
		return ""
	}
	j := i
	for j < len(b) && b[j] >= '0' && b[j] <= '9' {
		j++
	}
	return string(b[i:j])
}

// forumTopics maps a thread id to the name its forum_topic_created service
// message gives it. The thread id precedes the block, often in the same
// reply_to_message.
func forumTopics(b []byte) map[string]string {
	topics := map[string]string{}
	key := []byte("forum_topic_created")
	for from := 0; ; {
		i := bytes.Index(b[from:], key)
		if i < 0 {
			return topics
		}
		i += from
		name := topicNameAt(b[i+len(key):])
		id := threadBefore(b, i)
		if id != "" && name != "" && topics[id] == "" {
			topics[id] = name
		}
		from = i + len(key)
	}
}

func topicNameAt(b []byte) string {
	if len(b) > 240 {
		b = b[:240]
	}
	i := bytes.Index(b, []byte("name"))
	if i < 0 {
		return ""
	}
	rest := b[i+len("name"):]
	colon := bytes.IndexByte(rest, ':')
	if colon < 0 || colon > 12 {
		return ""
	}
	rest = bytes.TrimLeft(rest[colon+1:], " \t")
	for len(rest) > 0 && rest[0] == '\\' {
		rest = rest[1:]
	}
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	out := make([]byte, 0, 32)
	for i := 0; i < len(rest) && i < 120; i++ {
		if rest[i] == '\\' && i+1 < len(rest) {
			if rest[i+1] == '"' { // the value is itself JSON embedded as a string
				return strings.TrimSpace(string(out))
			}
			i++
			switch rest[i] {
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case 'r':
				out = append(out, '\r')
			default:
				out = append(out, rest[i])
			}
			continue
		}
		if rest[i] == '"' {
			return strings.TrimSpace(string(out))
		}
		out = append(out, rest[i])
	}
	return strings.TrimSpace(string(out))
}

// threadBefore is the message_thread_id nearest to a forum_topic_created
// block, i.e. the topic that block created.
func threadBefore(b []byte, end int) string {
	start := end - 400
	if start < 0 {
		start = 0
	}
	key := []byte("message_thread_id")
	for end > start {
		j := bytes.LastIndex(b[start:end], key)
		if j < 0 {
			return ""
		}
		j += start
		if id := threadAt(b[j+len(key):]); id != "" {
			return id
		}
		end = j
	}
	return ""
}

func mostCounted(counts map[string]int, order []string) (string, int) {
	picked, n := "", 0
	for _, key := range order {
		if counts[key] > n {
			picked, n = key, counts[key]
		}
	}
	return picked, n
}

func folderBase(folder string) string {
	folder = strings.ReplaceAll(folder, "/", `\`)
	if i := strings.LastIndex(folder, `\`); i >= 0 {
		return folder[i+1:]
	}
	return folder
}
