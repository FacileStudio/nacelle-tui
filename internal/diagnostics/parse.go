package diagnostics

import (
	"regexp"
	"strings"
)

type finding struct {
	loc  string
	rule string
	msg  string
}

var lineRe = regexp.MustCompile(`^(.+?):(\d+):(\d+): error: (.*) \[([a-z0-9._-]+)\]$`)

func parse(raw string) []finding {
	var fs []finding
	for line := range strings.SplitSeq(raw, "\n") {
		m := lineRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		fs = append(fs, finding{loc: m[1] + ":" + m[2] + ":" + m[3], rule: m[5], msg: m[4]})
	}
	return fs
}

func (f finding) key() string {
	return f.rule + ":" + f.msg
}

func (f finding) line() string {
	return f.loc + ": " + f.msg
}
