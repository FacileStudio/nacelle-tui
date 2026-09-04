package cost

import (
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/nacelle"
)

func TestSummaryReportsTotals(t *testing.T) {
	total := nacelle.Usage{
		InputTokens:     1000,
		OutputTokens:    1000,
		CacheReadTokens: 2000,
		Cost:            0.03,
	}
	tools, failed := 3, 1
	got := Summary(total, tools, failed, 90*time.Second)

	if !strings.Contains(got, "in 1.0k") || !strings.Contains(got, "out 1.0k") {
		t.Errorf("got = %q, want token totals", got)
	}
	if !strings.Contains(got, "2.0k cached") {
		t.Errorf("got = %q, want cache reads", got)
	}
	if !strings.Contains(got, "$0.0300") {
		t.Errorf("got = %q, want formatted cost", got)
	}
	if !strings.Contains(got, "3 tools · 1 failed") {
		t.Errorf("got = %q, want tool counts", got)
	}
	if !strings.Contains(got, "session · 1m30s") {
		t.Errorf("got = %q, want session duration", got)
	}
}

func TestSummaryOnEmptySession(t *testing.T) {
	got := Summary(nacelle.Usage{}, 0, 0, 5*time.Second)
	if strings.Contains(got, "$") || strings.Contains(got, "tool") {
		t.Errorf("got = %q, want no cost and no tools", got)
	}
	if !strings.Contains(got, "session · 5s") {
		t.Errorf("got = %q, want session duration", got)
	}
}

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0k"},
		{9900, "9.9k"},
		{10000, "10k"},
		{100000, "100k"},
	}
	for _, tc := range cases {
		if got := FormatTokens(tc.in); got != tc.want {
			t.Errorf("FormatTokens(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatCost(t *testing.T) {
	if got := FormatCost(0.0123); got != "$0.0123" {
		t.Errorf("FormatCost(0.0123) = %q, want $0.0123", got)
	}
}

func TestFormatDuration(t *testing.T) {
	if got := FormatDuration(0); got != "1s" {
		t.Errorf("FormatDuration(0) = %q, want 1s", got)
	}
	if got := FormatDuration(90 * time.Second); got != "1m30s" {
		t.Errorf("FormatDuration(90s) = %q, want 1m30s", got)
	}
}
