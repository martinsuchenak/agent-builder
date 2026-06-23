package compile

import (
	"fmt"
	"strings"
)

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
