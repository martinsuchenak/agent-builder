package install

import (
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	t.Setenv("HOME", "/home/test")
	cases := map[string]string{
		"":          "",
		"literal":   "literal",
		"~":         "/home/test",
		"~/foo":     filepath.Join("/home/test", "foo"),
		"~other":    "~other",
		"/abs/path": "/abs/path",
	}
	for in, want := range cases {
		if got := expandHome(in); got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}
