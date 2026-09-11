package diff

import (
	"slices"
	"strings"
	"testing"
)

func TestDiffLineNumsTracksBothFiles(t *testing.T) {
	ops := []diffOp{
		{kind: ' ', text: "a"},
		{kind: '-', text: "b"},
		{kind: '+', text: "B"},
		{kind: ' ', text: "c"},
	}
	nums := diffLineNums(ops)

	want := []int{1, 2, 2, 3}
	if !slices.Equal(nums, want) {
		t.Errorf("diffLineNums = %v, want %v (removals the old line, the rest new)", nums, want)
	}
}

func TestDiffLineNumsCountsALargeReplacement(t *testing.T) {
	before := strings.Repeat("r\n", 100)
	after := strings.Repeat("a\n", 200)
	nums := diffLineNums(diffOps(splitLines(before), splitLines(after)))
	if len(nums) != 300 || nums[0] != 1 || nums[99] != 100 || nums[100] != 1 || nums[299] != 200 {
		t.Errorf("diffLineNums mis-numbered a replacement, want old 1..100 then new 1..200")
	}
	if got := gutterWidth(nums); got != 3 {
		t.Errorf("gutterWidth = %d, want 3 for a 200-line number", got)
	}
}

func TestGutterRightAlignsNumberAndMarker(t *testing.T) {
	if gutter(3, 2, "-") != "  3 - " {
		t.Errorf("gutter = %q, want a space, then the right-aligned number before the marker", gutter(3, 2, "-"))
	}
	if gutter(5, 1, " ") != " 5   " {
		t.Errorf("gutter = %q, want the marker column open on context", gutter(5, 1, " "))
	}
}
