package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCindyChannel(t *testing.T) {
	for _, test := range []struct {
		folder, body string
		want         cindyLabel
	}{
		{"C:/work/app", "帮我改一下这个项目", cindyLabel{Channel: "电脑端"}},
		{"C:/work/app", "[客户端说明] 系统追加。用户正在一台电脑上远程操作。", cindyLabel{Channel: "电脑端"}},
		{
			`C:\im-working-dir\telegram-9085529043`,
			"本条来自 Telegram 私聊 (chat_id: 5159416756)。\\n你的名字: ohmycindy\\n\\n",
			cindyLabel{Channel: "Telegram Bot · ohmycindy"},
		},
		{
			`C:\im-working-dir\telegram-9085529043`,
			"本条来自 Telegram 私聊 (chat_id: 42)。\n你的名字：cindyapp\n",
			cindyLabel{Channel: "Telegram Bot · cindyapp"},
		},
		{
			"",
			"本条来自 Telegram 私聊 (chat_id: 5159416756)。",
			cindyLabel{Channel: "Telegram Bot · 5159416756"},
		},
		{
			"",
			"本条来自 Telegram 群「forcindy」(chat_id: -1003901021780)。话题「其他」话题「推片」话题「推片」",
			cindyLabel{Channel: "Telegram 群 · forcindy"},
		},
		{
			"",
			"本条来自 Telegram 群「forcindy」(chat_id: -100)。message_thread_id\": 8 message_thread_id\": 215 message_thread_id\": 215 message_thread_id!==void 0",
			cindyLabel{Channel: "Telegram 群 · forcindy", TopicID: "215", Topic: "话题 215"},
		},
		{
			"",
			"本条来自 Telegram 群「forcindy」(chat_id: -100)。forum_topic_created\":{\"name\":\"日本影视\"}",
			cindyLabel{Channel: "Telegram 群 · forcindy"},
		},
		{
			"",
			`本条来自 Telegram 群「forcindy」(chat_id: -100)。"message_thread_id": 215, "forum_topic_created":{"name":"日本影视作品"}`,
			cindyLabel{Channel: "Telegram 群 · forcindy", TopicID: "215", Topic: "日本影视作品"},
		},
		{
			"",
			`本条来自 Telegram 群「forcindy」(chat_id: -100)。"message_thread_id": 215, \"forum_topic_created\": {\"name\": \"日本影视作品\", \"icon_color\": 1}`,
			cindyLabel{Channel: "Telegram 群 · forcindy", TopicID: "215", Topic: "日本影视作品"},
		},
		{
			"",
			"本条来自 Telegram 群「forcindy」(chat_id: -100)。话题「存图专用」",
			cindyLabel{Channel: "Telegram 群 · forcindy"},
		},
		{
			"",
			"本条来自 Discord 群「forcindy #japan」(chat_id: 1571710619932041216)。",
			cindyLabel{Channel: "Discord · forcindy #japan"},
		},
		{
			"",
			"本条来自 Discord 私聊 (chat_id: 1571708764841705562)。",
			cindyLabel{Channel: "Discord 私聊"},
		},
		{`C:\im-working-dir\telegram-9085529043`, "", cindyLabel{Channel: "Telegram Bot · 9085529043"}},
		{`C:\im-working-dir\discord-1571707155457908806`, "", cindyLabel{Channel: "Discord · discord-1571707155457908806"}},
	} {
		got := cindyChannel(test.folder, []byte(test.body), "", map[string]string{})
		if got != test.want {
			t.Errorf("cindyChannel(%q) = %+v, want %+v", test.body, got, test.want)
		}
	}
}

func TestCindyTopicFromFirstReply(t *testing.T) {
	body := []byte(`本条来自 Telegram 群「forcindy」(chat_id: -1003901021780)。
<reply_context>
[tom s] #47 2026-11-16 03:40 UTC [引用消息的正文无法读取]
</reply_context>`)
	id, topic := groupTopic(body, "-1003901021780", map[string]bool{"47": true}, map[string]string{})
	if id != "47" || topic != "话题 47" {
		t.Fatalf("groupTopic() = %q, %q; want 47, 话题 47", id, topic)
	}
}

func TestCindyTopicNameIsShared(t *testing.T) {
	names := map[string]string{}
	withName := []byte(`本条来自 Telegram 群「forcindy」(chat_id: -100)。"message_thread_id": 215, forum_topic_created": {"name": "日本影视作品"}`)
	if id, topic := groupTopic(withName, "-100", nil, names); id != "215" || topic != "日本影视作品" {
		t.Fatalf("first groupTopic() = %q, %q", id, topic)
	}
	withoutName := []byte(`本条来自 Telegram 群「forcindy」(chat_id: -100)。"message_thread_id": 215`)
	if id, topic := groupTopic(withoutName, "-100", nil, names); id != "215" || topic != "日本影视作品" {
		t.Fatalf("second groupTopic() = %q, %q", id, topic)
	}
}

func TestCindyTopicFromRuntime(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "pi-agent-home", "sessions")
	runtime := filepath.Join(root, "pi-agent-home", "runtime")
	for _, dir := range []string{sessions, runtime} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(runtime, "perm-telegram_9085529043_g--1003901021780-47-0123456789abcdef.json"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	body := []byte(`本条来自 Telegram 群「forcindy」(chat_id: -1003901021780)。你的名字: ohmycindy
<reply_context>
[tom s] #47 2026-11-16 03:40 UTC [引用消息的正文无法读取]
</reply_context>`)
	got := cindyChannel(`C:\im-working-dir\telegram-9085529043`, body, filepath.Join(sessions, "s.jsonl"), map[string]string{})
	want := cindyLabel{Channel: "Telegram 群 · forcindy", TopicID: "47", Topic: "话题 47"}
	if got != want {
		t.Fatalf("cindyChannel() = %+v, want %+v", got, want)
	}
}

func TestCindyTopicIDs(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "pi-agent-home", "sessions")
	runtime := filepath.Join(root, "pi-agent-home", "runtime")
	for _, dir := range []string{sessions, runtime} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{
		"mcp-disclosure-telegram_9085529043_g--1003901021780-47.json",
		"perm-telegram_9085529043_g--1003901021780-215-0123456789abcdef.json",
		"mcp-disclosure-telegram_9085529043_g--1003901021780.json",
	} {
		if err := os.WriteFile(filepath.Join(runtime, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ids := cindyTopicIDs(filepath.Join(sessions, "session.jsonl"), "9085529043", "-1003901021780")
	if !ids["47"] || !ids["215"] || len(ids) != 2 {
		t.Fatalf("cindyTopicIDs() = %v, want 47 and 215", ids)
	}
}
