package main

import (
	"errors"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/sessions"
)

type dateRange struct {
	start time.Time
	end   time.Time
}

func parseDateRange(from, to string) (dateRange, error) {
	parse := func(value string) (time.Time, error) {
		if value == "" {
			return time.Time{}, nil
		}
		day, err := time.ParseInLocation(time.DateOnly, value, time.Local)
		if err != nil || day.Year() < 1 {
			return time.Time{}, errors.New("请输入有效日期（YYYY-MM-DD）")
		}
		return day, nil
	}
	start, err := parse(from)
	if err != nil {
		return dateRange{}, err
	}
	end, err := parse(to)
	if err != nil {
		return dateRange{}, err
	}
	if !end.IsZero() {
		if end.Before(start) {
			return dateRange{}, errors.New("结束日期不能早于开始日期")
		}
		end = end.AddDate(0, 0, 1)
	}
	return dateRange{start, end}, nil
}

func (d dateRange) active() bool { return !d.start.IsZero() || !d.end.IsZero() }

func (d dateRange) includes(at time.Time) bool {
	return !at.IsZero() && !at.Before(d.start) && (d.end.IsZero() || at.Before(d.end))
}

func (d dateRange) parts(s sessions.Session, each func(time.Time, sessions.Part) bool) error {
	return sessions.FileFeedDatedParts(s, func(at time.Time, part sessions.Part) bool {
		if !d.includes(at) || strings.TrimSpace(part.Text) == "" {
			return true
		}
		return each(at, part)
	})
}

var errNoDatedContent = errors.New("该日期范围内没有可导出的内容")

type dateMatches struct {
	Rows    []sessionRow `json:"rows"`
	Skipped int          `json:"skipped"`
}

func listDateMatches(agent string, dates dateRange) dateMatches {
	result := dateMatches{Rows: []sessionRow{}}
	names := loadTopicNames()
	for _, s := range localSessions(agent) {
		if agent != "cindy" {
			if dates.includes(s.Start) {
				result.Rows = append(result.Rows, row(s, names))
			}
			continue
		}
		var latest time.Time
		if err := dates.parts(s.Session, func(at time.Time, _ sessions.Part) bool {
			if at.After(latest) {
				latest = at
			}
			return true
		}); err != nil {
			result.Skipped++
			continue
		}
		if !latest.IsZero() {
			matched := row(s, names)
			matched.RangeLast = latest.Local().Format(time.RFC3339)
			result.Rows = append(result.Rows, matched)
		}
	}
	return result
}
