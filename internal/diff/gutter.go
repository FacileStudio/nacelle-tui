package diff

import (
	"strconv"
	"strings"
)

// diffLineNums numbers each operation the way a source gutter does: a removal
// keeps its old-file line, context and additions keep the new-file line, and
// both counts run through the whole change so the numbers stay correct across
// the gaps a display cuts between hunks.
func diffLineNums(ops []diffOp) []int {
	oldN, newN := 1, 1
	nums := make([]int, len(ops))
	for i, op := range ops {
		switch op.kind {
		case '-':
			nums[i] = oldN
			oldN++
		case '+':
			nums[i] = newN
			newN++
		default:
			nums[i] = newN
			oldN++
			newN++
		}
	}
	return nums
}

// gutterWidth is how wide the line-number column must be to hold the biggest
// number in the change, so every code row opens in the same column.
func gutterWidth(nums []int) int {
	maxNo := 0
	for _, n := range nums {
		if n > maxNo {
			maxNo = n
		}
	}
	return len(strconv.Itoa(maxNo))
}

// gutter lays out one row's number at the right of the gutter, then the diff
// marker and a space, so the marker column lines up down the whole pane.
func gutter(no, ln int, marker string) string {
	num := strconv.Itoa(no)
	return strings.Repeat(" ", ln-len(num)) + num + " " + marker + " "
}
