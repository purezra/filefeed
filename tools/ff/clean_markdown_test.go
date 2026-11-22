package main

import (
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/sessions"
)

func TestCleanMarkdownKeepsQuestionsAndAnswers(t *testing.T) {
	parts := []sessions.Part{
		{Role: "user", Kind: "context", Text: "<environment_context>private paths</environment_context>\n真实提问"},
		{Role: "user", Kind: "image", Text: "attachment"},
		{Role: "assistant", Kind: "thinking", Text: "internal reasoning\n```\n</details>"},
		{Role: "assistant", Kind: "tool_use", Text: "arguments"},
		{Role: "tool", Kind: "tool_result", Text: "private result"},
		{Role: "assistant", Kind: "text", Text: "公开答案\n```md\n## User"},
		{Role: "user", Kind: "text", Text: "第二个问题"},
		{Role: "assistant", Kind: "text", Text: "第二个答案"},
	}
	var out strings.Builder
	err := writeCleanMarkdown(&out, sessions.Session{Title: "Example", Agent: "codex", ID: "id"}, func(each func(sessions.Part) bool) error {
		for _, part := range parts {
			if !each(part) {
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	clean := out.String()
	for _, text := range []string{"# Example", "## Q1\n\n真实提问", "## A1\n\n公开答案", "```md\n## User", "## Q2\n\n第二个问题", "## A2\n\n第二个答案"} {
		if !strings.Contains(clean, text) {
			t.Errorf("missing %q", text)
		}
	}
	for _, text := range []string{"private paths", "attachment", "internal reasoning", "arguments", "private result", "</details>"} {
		if strings.Contains(clean, text) {
			t.Errorf("unexpected %q", text)
		}
	}
}
