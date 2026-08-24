package main

// lcsCellLimit caps the dynamic-programming table at roughly four million
// cells. Beyond it the middle of the two texts is shown as one wholesale
// replacement rather than paired up line by line — coarser, but never wrong,
// and reached only by files nobody diffs to read line by line.
const lcsCellLimit = 4_000_000

// diffOp is one line of a diff and what happened to it: kept, removed or
// added, marked ' ', '-' and '+' respectively.
type diffOp struct {
	kind byte
	text string
}

// diffOps pairs the two texts' lines up into kept, removed and added lines.
//
// Common lines at either end are trimmed before any work is done, which is
// both the cheap win and what keeps a one-line edit to a large file inside
// the table limit.
func diffOps(before, after []string) []diffOp {
	prefix, suffix := sharedEnds(before, after)

	ops := keptLines(before[:prefix])
	ops = append(ops, middleOps(before[prefix:len(before)-suffix], after[prefix:len(after)-suffix])...)
	return append(ops, keptLines(after[len(after)-suffix:])...)
}

// sharedEnds is how many leading and how many trailing lines the two texts
// share, the prefix taken first so the two cannot claim the same line in a
// text that is nothing but repeats.
func sharedEnds(before, after []string) (int, int) {
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(before)-prefix && suffix < len(after)-prefix &&
		before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	return prefix, suffix
}

// keptLines marks a run of unchanged lines as context.
func keptLines(lines []string) []diffOp {
	ops := make([]diffOp, 0, len(lines))
	for _, line := range lines {
		ops = append(ops, diffOp{kind: ' ', text: line})
	}
	return ops
}

// middleOps aligns the changed middles of the two texts: paired up line by
// line when they are small enough for the table, and as one wholesale
// replacement when they are not.
func middleOps(before, after []string) []diffOp {
	if len(before)*len(after) > lcsCellLimit {
		return replacedLines(before, after)
	}
	return pairUp(before, after)
}

// replacedLines reports a middle too large to align as removals against
// additions, interleaved so both sides survive any display cap that follows.
func replacedLines(before, after []string) []diffOp {
	ops := make([]diffOp, 0, len(before)+len(after))
	for i := 0; i < max(len(before), len(after)); i++ {
		if i < len(before) {
			ops = append(ops, diffOp{kind: '-', text: before[i]})
		}
		if i < len(after) {
			ops = append(ops, diffOp{kind: '+', text: after[i]})
		}
	}
	return ops
}

// pairUp aligns two changed middles by longest common subsequence: the table
// counts the matches reachable from each cell, and the walk out of its origin
// turns those counts into kept, removed and added lines.
func pairUp(before, after []string) []diffOp {
	table := lcsTable(before, after)

	ops := make([]diffOp, 0, len(before)+len(after))
	for i, j := 0, 0; i < len(before) || j < len(after); {
		switch {
		case i < len(before) && j < len(after) && before[i] == after[j]:
			ops = append(ops, diffOp{kind: ' ', text: before[i]})
			i++
			j++
		case j == len(after) || (i < len(before) && table[i+1][j] >= table[i][j+1]):
			ops = append(ops, diffOp{kind: '-', text: before[i]})
			i++
		default:
			ops = append(ops, diffOp{kind: '+', text: after[j]})
			j++
		}
	}
	return ops
}

// lcsTable builds the longest-common-subsequence length table for two runs of
// lines, with a spare row and column of zeros so the walk never tests a bound.
func lcsTable(before, after []string) [][]int {
	table := make([][]int, len(before)+1)
	for i := range table {
		table[i] = make([]int, len(after)+1)
	}
	for i := len(before) - 1; i >= 0; i-- {
		for j := len(after) - 1; j >= 0; j-- {
			if before[i] == after[j] {
				table[i][j] = table[i+1][j+1] + 1
				continue
			}
			table[i][j] = max(table[i+1][j], table[i][j+1])
		}
	}
	return table
}

// hunks groups diff operations into the blocks actually shown: each change,
// padded with up to context unchanged neighbours, its neighbourhood merged
// with the next change's when the two overlap.
func hunks(ops []diffOp, context int) [][]diffOp {
	var ranges [][2]int
	for at, op := range ops {
		if op.kind == ' ' {
			continue
		}
		start, end := max(at-context, 0), min(at+context, len(ops)-1)
		if last := len(ranges) - 1; last >= 0 && start <= ranges[last][1]+1 {
			ranges[last][1] = max(ranges[last][1], end)
			continue
		}
		ranges = append(ranges, [2]int{start, end})
	}

	blocks := make([][]diffOp, 0, len(ranges))
	for _, r := range ranges {
		blocks = append(blocks, ops[r[0]:r[1]+1])
	}
	return blocks
}
