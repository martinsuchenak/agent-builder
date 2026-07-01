package install_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-builder/internal/install"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func buildLayout(t *testing.T, target string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		writeFile(t, filepath.Join(root, target, rel), content)
	}
	return root
}

func actionsOf(res []install.FileResult) []install.Action {
	out := make([]install.Action, len(res))
	for i, r := range res {
		out[i] = r.Action
	}
	return out
}

func TestCreatedAndDestRaw(t *testing.T) {
	build := buildLayout(t, "claude", map[string]string{".claude/commands/x.md": "new"})
	dest := t.TempDir()
	res, err := install.Run(install.Plan{BuildRoot: build, Targets: []string{"claude"}, Dest: dest, NonInteractive: true, Out: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Action != install.Created {
		t.Fatalf("expected one Created, got %+v", res)
	}
	got, _ := os.ReadFile(filepath.Join(dest, ".claude", "commands", "x.md"))
	if string(got) != "new" {
		t.Errorf("content wrong: %q", got)
	}
}

func TestForceOverwrite(t *testing.T) {
	build := buildLayout(t, "claude", map[string]string{".claude/commands/x.md": "new"})
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, ".claude", "commands", "x.md"), "old")
	res, _ := install.Run(install.Plan{BuildRoot: build, Targets: []string{"claude"}, Dest: dest, Force: true, Out: io.Discard})
	if len(res) != 1 || res[0].Action != install.Overwritten {
		t.Fatalf("expected Overwritten, got %+v", res)
	}
	got, _ := os.ReadFile(filepath.Join(dest, ".claude", "commands", "x.md"))
	if string(got) != "new" {
		t.Errorf("should be overwritten with new: %q", got)
	}
}

func TestNonInteractiveSkips(t *testing.T) {
	build := buildLayout(t, "claude", map[string]string{".claude/commands/x.md": "new"})
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, ".claude", "commands", "x.md"), "old")
	res, _ := install.Run(install.Plan{BuildRoot: build, Targets: []string{"claude"}, Dest: dest, NonInteractive: true, Out: io.Discard})
	if len(res) != 1 || res[0].Action != install.Skipped {
		t.Fatalf("expected Skipped, got %+v", res)
	}
	got, _ := os.ReadFile(filepath.Join(dest, ".claude", "commands", "x.md"))
	if string(got) != "old" {
		t.Errorf("existing file should be untouched: %q", got)
	}
}

func TestPromptYesNoAllQuit(t *testing.T) {
	build := buildLayout(t, "claude", map[string]string{
		".claude/commands/a.md": "A", ".claude/commands/b.md": "B",
	})
	cases := []struct {
		in      string
		want    []install.Action
		aborted bool
	}{
		{"y\n", []install.Action{install.Overwritten, install.Skipped}, false},
		{"n\n", []install.Action{install.Skipped, install.Skipped}, false},
		{"a\n", []install.Action{install.Overwritten, install.Overwritten}, false},
		{"q\n", []install.Action{}, true},
	}
	for _, c := range cases {
		dest := t.TempDir()
		writeFile(t, filepath.Join(dest, ".claude", "commands", "a.md"), "old")
		writeFile(t, filepath.Join(dest, ".claude", "commands", "b.md"), "old")
		res, err := install.Run(install.Plan{
			BuildRoot: build, Targets: []string{"claude"}, Dest: dest,
			In: strings.NewReader(c.in), Out: io.Discard,
		})
		if err != nil {
			t.Errorf("input %q: user abort should not surface as an error, got %v", c.in, err)
		}
		if c.aborted {
			if len(res) != 0 {
				t.Errorf("input %q: expected abort (no results), got %+v", c.in, res)
			}
			continue
		}
		if len(res) != len(c.want) {
			t.Errorf("input %q: got %d results, want %d", c.in, len(res), len(c.want))
			continue
		}
		for i, a := range actionsOf(res) {
			if a != c.want[i] {
				t.Errorf("input %q: result[%d] = %v, want %v", c.in, i, a, c.want[i])
			}
		}
	}
}

func TestManagedMergePreservesHandContent(t *testing.T) {
	compiled := "<!-- BEGIN ab:rule r1 -->\nrule body\n<!-- END ab:rule r1 -->\n"
	build := buildLayout(t, "claude", map[string]string{"CLAUDE.md": compiled})
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, "CLAUDE.md"), "hand written intro\n")
	res, err := install.Run(install.Plan{BuildRoot: build, Targets: []string{"claude"}, Dest: dest, NonInteractive: true, Out: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Action != install.Merged {
		t.Fatalf("expected Merged, got %+v", res)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "CLAUDE.md"))
	s := string(got)
	if !strings.Contains(s, "hand written intro") || !strings.Contains(s, "rule body") {
		t.Errorf("merge lost content:\n%s", s)
	}
}

func TestActionString(t *testing.T) {
	want := map[install.Action]string{
		install.Created: "created", install.Overwritten: "overwrote",
		install.Skipped: "skipped", install.Merged: "merged",
	}
	for a, s := range want {
		if a.String() != s {
			t.Errorf("%v String() = %q, want %q", a, a.String(), s)
		}
	}
	if install.Action(99).String() != "?" {
		t.Error("unknown action should render ?")
	}
}

func TestDefaultDestStripsPrefixToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	build := buildLayout(t, "claude", map[string]string{".claude/commands/x.md": "new"})
	res, err := install.Run(install.Plan{BuildRoot: build, Targets: []string{"claude"}, NonInteractive: true, Out: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Action != install.Created {
		t.Fatalf("expected Created, got %+v", res)
	}
	dest := filepath.Join(home, ".claude", "commands", "x.md")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected installed at %s: %v", dest, err)
	}
}
