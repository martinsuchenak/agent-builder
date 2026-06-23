package compile

import (
	"fmt"
	"regexp"
	"strings"
)

// regionRe matches a managed region. RE2 has no backreferences, so the END
// marker is matched generically; bodies are non-greedy and regions do not
// nest, so the first END after a BEGIN is its match.
var regionRe = regexp.MustCompile(`(?s)<!-- BEGIN ab:(\w+) (\S+) -->(.*?)<!-- END ab:\w+ \S+ -->`)

// MergeManagedRegions applies every managed region found in compiled into dest,
// preserving any hand-written content outside the regions. Used when installing
// a managed rules file (CLAUDE.md / AGENTS.md) over an existing one.
func MergeManagedRegions(dest, compiled []byte) []byte {
	out := dest
	for _, m := range regionRe.FindAllSubmatch(compiled, -1) {
		out = MergeManaged(out, string(m[1]), string(m[2]), strings.Trim(string(m[3]), "\n"))
	}
	return out
}

// MergeManaged merges body into existing under a managed region delimited by
// HTML comment markers named for namespace and id. Re-running replaces the
// region in place rather than duplicating it.
func MergeManaged(existing []byte, namespace, id, body string) []byte {
	begin := fmt.Sprintf("<!-- BEGIN ab:%s %s -->", namespace, id)
	end := fmt.Sprintf("<!-- END ab:%s %s -->", namespace, id)
	block := begin + "\n" + strings.TrimRight(body, "\n") + "\n" + end

	s := string(existing)
	if bi := strings.Index(s, begin); bi >= 0 {
		if ei := strings.Index(s[bi:], end); ei >= 0 {
			ei += bi + len(end)
			for ei < len(s) && s[ei] == '\n' {
				ei++
			}
			if bi > 0 && s[bi-1] == '\n' {
				bi--
			}
			return []byte(s[:bi] + block + "\n" + s[ei:])
		}
	}
	if len(s) > 0 && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	if len(s) > 0 {
		s += "\n"
	}
	s += block + "\n"
	return []byte(s)
}
