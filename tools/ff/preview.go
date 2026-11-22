package main

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yetone/magpie/internal/sessions"
)

const previewSize = 100
const previewPartSize = 8000

type sessionPreview struct {
	Parts  []sessions.Part `json:"parts"`
	Before int             `json:"before"`
}

func readPreview(s sessions.Session, clean bool, dates dateRange, before int) (sessionPreview, error) {
	var ring [previewSize]sessions.Part
	total := 0
	each := func(part sessions.Part) bool {
		if clean && ((part.Role != "user" && part.Role != "assistant") || (part.Kind != "text" && part.Kind != "image")) {
			return true
		}
		part.Text = strings.TrimSpace(part.Text)
		if part.Text == "" {
			return true
		}
		if before > 0 && total == before {
			return false
		}
		if count := utf8.RuneCountInString(part.Text); count > previewPartSize {
			part.Text = string([]rune(part.Text)[:previewPartSize])
			part.Cut += count - previewPartSize
		}
		ring[total%previewSize] = part
		total++
		return true
	}
	var err error
	if dates.active() && s.Agent == "cindy" {
		err = dates.parts(s, func(_ time.Time, part sessions.Part) bool { return each(part) })
	} else {
		err = sessions.FileFeedParts(s, each)
	}
	start := max(0, total-previewSize)
	out := sessionPreview{Parts: make([]sessions.Part, 0, total-start), Before: start}
	for i := start; i < total; i++ {
		out.Parts = append(out.Parts, ring[i%previewSize])
	}
	return out, err
}
