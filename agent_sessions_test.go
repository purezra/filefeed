package main

import (
	"testing"
	"time"
)

func TestSessionExportFilename(t *testing.T) {
	local := time.FixedZone("UTC+8", 8*60*60)
	for _, test := range []struct {
		agent string
		at    time.Time
		clean bool
		want  string
	}{
		{"Pi", time.Date(2026, 11, 21, 15, 23, 42, 0, local), true, "pi_202611211523_filefeed_clean.md"},
		{"Claude Code", time.Date(2026, 11, 11, 0, 21, 0, 0, local), false, "claudecode_202611110021_filefeed.md"},
	} {
		if got := sessionExportFilename(test.agent, test.at, test.clean); got != test.want {
			t.Errorf("filename for %q = %q, want %q", test.agent, got, test.want)
		}
	}
}
