package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(fn func() int) (string, int) {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := fn()
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out), code
}

func TestParseFlags(t *testing.T) {
	var src, kind string
	var targets []string
	var cont, verb bool
	pos, err := parseFlags([]string{"--target", "a", "--target=b", "--source", "s", "--kind", "k", "--continue", "pos1", "pos2"},
		map[string]*[]string{"target": &targets},
		map[string]*string{"source": &src, "kind": &kind},
		map[string]*bool{"continue": &cont, "verbose": &verb})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0] != "a" || targets[1] != "b" {
		t.Errorf("targets: %v", targets)
	}
	if src != "s" || kind != "k" || !cont {
		t.Errorf("singles/bools: src=%q kind=%q cont=%v", src, kind, cont)
	}
	if len(pos) != 2 || pos[0] != "pos1" {
		t.Errorf("positional: %v", pos)
	}
	if _, err := parseFlags([]string{"--bogus", "x"}, nil, nil, nil); err == nil {
		t.Error("unknown flag should error")
	}
	if _, err := parseFlags([]string{"--source"}, nil, map[string]*string{"source": &src}, nil); err == nil {
		t.Error("missing value should error")
	}
}

func TestRunTargets(t *testing.T) {
	out, code := captureStdout(func() int { return runTargets(nil) })
	if code != 0 {
		t.Fatalf("code %d", code)
	}
	for _, tgt := range []string{"claude", "opencode", "codex", "copilot", "kiro"} {
		if !strings.Contains(out, tgt) {
			t.Errorf("missing target %s in:\n%s", tgt, out)
		}
	}
}

func makeSource(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "artifacts", "commands"), 0o755)
	os.WriteFile(filepath.Join(src, "artifacts", "commands", "x.md"),
		[]byte("---\nid: x\nkind: command\ndescription: d\ntargets: [claude]\n---\nhello {{input}}"), 0o644)
	return src
}

func TestRunValidateAndCompile(t *testing.T) {
	src := makeSource(t)
	out, code := captureStdout(func() int { return runValidate([]string{"--source", src}) })
	if code != 0 || !strings.Contains(out, "OK") {
		t.Fatalf("validate valid source: code=%d out=%s", code, out)
	}

	outDir := t.TempDir()
	_, code = captureStdout(func() int {
		return runCompile([]string{"--source", src, "--out", outDir, "--target", "claude", "--non-interactive"})
	})
	if code != 0 {
		t.Fatalf("compile failed: code %d", code)
	}
	if _, err := os.Stat(filepath.Join(outDir, "claude", ".claude", "commands", "x.md")); err != nil {
		t.Errorf("compiled command not written: %v", err)
	}
}

func TestRunValidateInvalid(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "artifacts", "commands"), 0o755)
	os.WriteFile(filepath.Join(src, "artifacts", "commands", "bad.md"),
		[]byte("---\nid: bad\nkind: command\n---\n{{tool unknown}}"), 0o644)
	_, code := captureStdout(func() int { return runValidate([]string{"--source", src}) })
	if code != 1 {
		t.Fatalf("invalid source should return 1, got %d", code)
	}
}

func TestRunNew(t *testing.T) {
	src := t.TempDir()
	_, code := captureStdout(func() int { return runNew([]string{"command", "fresh", "--source", src}) })
	if code != 0 {
		t.Fatalf("new command failed: %d", code)
	}
	if _, err := os.Stat(filepath.Join(src, "artifacts", "commands", "fresh.md")); err != nil {
		t.Errorf("scaffolded file missing: %v", err)
	}
	_, code = captureStdout(func() int { return runNew([]string{"command", "fresh", "--source", src}) })
	if code != 1 {
		t.Errorf("duplicate new should return 1, got %d", code)
	}
	_, code = captureStdout(func() int { return runNew([]string{"boguskind", "x", "--source", src}) })
	if code != 2 {
		t.Errorf("invalid kind should return 2, got %d", code)
	}
	_, code = captureStdout(func() int { return runNew([]string{"command"}) })
	if code != 2 {
		t.Errorf("missing args should return 2, got %d", code)
	}
}

func TestPositionalPaths(t *testing.T) {
	src := makeSource(t)
	out, code := captureStdout(func() int { return runValidate([]string{src}) })
	if code != 0 || !strings.Contains(out, "OK") {
		t.Fatalf("positional validate: code=%d out=%s", code, out)
	}

	src2 := makeSource(t)
	outDir := t.TempDir()
	_, code = captureStdout(func() int { return runCompile([]string{src2, outDir, "--non-interactive"}) })
	if code != 0 {
		t.Fatalf("positional compile failed: %d", code)
	}
	if _, err := os.Stat(filepath.Join(outDir, "claude", ".claude", "commands", "x.md")); err != nil {
		t.Errorf("positional out not used: %v", err)
	}
}

func TestRunNewFolderKinds(t *testing.T) {
	for _, tc := range []struct {
		kind, entry string
	}{
		{"skill", "SKILL.md"},
		{"power", "POWER.md"},
	} {
		src := t.TempDir()
		_, code := captureStdout(func() int { return runNew([]string{tc.kind, "thing", "--source", src}) })
		if code != 0 {
			t.Fatalf("%s: new failed: %d", tc.kind, code)
		}
		path := filepath.Join(src, "artifacts", tc.kind+"s", "thing", tc.entry)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		if !strings.Contains(string(data), "name: thing") {
			t.Errorf("%s scaffold missing name:\n%s", tc.kind, data)
		}
		if tc.kind == "power" && !strings.Contains(string(data), "displayName: thing") {
			t.Errorf("power scaffold missing displayName:\n%s", data)
		}
	}
}

func TestMainDispatch(t *testing.T) {
	out, code := captureStdout(func() int { return Main([]string{"help"}) })
	if code != 0 || !strings.Contains(out, "agent-builder") {
		t.Fatalf("help wrong: code=%d out=%s", code, out)
	}
	_, code = captureStdout(func() int { return Main([]string{"nope"}) })
	if code != 2 {
		t.Errorf("unknown command should return 2, got %d", code)
	}
	_, code = captureStdout(func() int { return Main(nil) })
	if code != 0 {
		t.Errorf("no args should print usage and return 0, got %d", code)
	}
}

func TestIsTerminalAndScaffold(t *testing.T) {
	if isTerminal(nil) {
		t.Error("nil file should not be a terminal")
	}
	s := scaffold("command", "cmd")
	if !strings.Contains(s, "id: cmd") || !strings.Contains(s, "{{tool ID@server k=v}}") {
		t.Errorf("scaffold template wrong:\n%s", s)
	}
}

func TestRunCompileBranches(t *testing.T) {
	badSrc := t.TempDir()
	os.MkdirAll(filepath.Join(badSrc, "artifacts", "commands"), 0o755)
	os.WriteFile(filepath.Join(badSrc, "artifacts", "commands", "bad.md"),
		[]byte("---\nid: bad\nkind: command\ntargets: [claude]\n---\n{{tool unknown}}"), 0o644)
	_, code := captureStdout(func() int {
		return runCompile([]string{"--source", badSrc, "--out", t.TempDir(), "--target", "claude", "--non-interactive"})
	})
	if code != 1 {
		t.Fatalf("failing compile should be 1, got %d", code)
	}

	skipSrc := t.TempDir()
	os.MkdirAll(filepath.Join(skipSrc, "artifacts", "agents"), 0o755)
	os.WriteFile(filepath.Join(skipSrc, "artifacts", "agents", "ag.md"),
		[]byte("---\nid: ag\nkind: agent\ndescription: d\ntargets: [kiro]\n---\nbody"), 0o644)
	out, code := captureStdout(func() int {
		return runCompile([]string{"--source", skipSrc, "--out", t.TempDir(), "--target", "kiro", "--continue", "--non-interactive"})
	})
	if code != 0 || !strings.Contains(out, "1 skipped") {
		t.Fatalf("skip case wrong: code=%d out=%s", code, out)
	}

	src := makeSource(t)
	_, code = captureStdout(func() int {
		return runCompile([]string{"--source", src, "--out", t.TempDir(), "--target", "claude", "--kind", "command", "--non-interactive"})
	})
	if code != 0 {
		t.Fatalf("kind-filter compile failed: %d", code)
	}
}

func TestRunInstall(t *testing.T) {
	build := t.TempDir()
	os.MkdirAll(filepath.Join(build, "claude", ".claude", "commands"), 0o755)
	os.WriteFile(filepath.Join(build, "claude", ".claude", "commands", "x.md"), []byte("x"), 0o644)
	dest := t.TempDir()
	out, code := captureStdout(func() int {
		return runInstall([]string{build, "--dest", dest, "--target", "claude", "--non-interactive"})
	})
	if code != 0 {
		t.Fatalf("install failed: %d", code)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("expected a created file in:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dest, ".claude", "commands", "x.md")); err != nil {
		t.Errorf("file not installed: %v", err)
	}
}

func TestRunInstallMergeAndSkip(t *testing.T) {
	build := t.TempDir()
	os.MkdirAll(filepath.Join(build, "claude", ".claude", "commands"), 0o755)
	os.WriteFile(filepath.Join(build, "claude", ".claude", "commands", "x.md"), []byte("new"), 0o644)
	os.WriteFile(filepath.Join(build, "claude", "CLAUDE.md"),
		[]byte("<!-- BEGIN ab:rule r1 -->\nbody\n<!-- END ab:rule r1 -->\n"), 0o644)

	dest := t.TempDir()
	os.MkdirAll(filepath.Join(dest, ".claude", "commands"), 0o755)
	os.WriteFile(filepath.Join(dest, ".claude", "commands", "x.md"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(dest, "CLAUDE.md"), []byte("hand content\n"), 0o644)

	out, code := captureStdout(func() int {
		return runInstall([]string{build, "--dest", dest, "--target", "claude", "--non-interactive"})
	})
	if code != 0 {
		t.Fatalf("install failed: %d", code)
	}
	if !strings.Contains(out, "1 merged") || !strings.Contains(out, "1 skipped") {
		t.Errorf("expected a merged + a skipped in:\n%s", out)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "CLAUDE.md"))
	if !strings.Contains(string(got), "hand content") || !strings.Contains(string(got), "body") {
		t.Errorf("CLAUDE.md not merged:\n%s", got)
	}
}

func TestRunValidateMultiTargetAndNewFlagError(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "artifacts", "commands"), 0o755)
	os.WriteFile(filepath.Join(src, "artifacts", "commands", "m.md"),
		[]byte("---\nid: m\nkind: command\ntargets: [claude, opencode]\n---\n{{tool echo@fortix x=1}}"), 0o644)
	out, code := captureStdout(func() int { return runValidate([]string{"--source", src}) })
	if code != 0 || !strings.Contains(out, "OK") {
		t.Fatalf("multi-target validate: code=%d out=%s", code, out)
	}

	_, code = captureStdout(func() int { return runNew([]string{"command", "x", "--bogus"}) })
	if code != 2 {
		t.Errorf("bad flag in new should return 2, got %d", code)
	}

	_, code = captureStdout(func() int { return Main([]string{"-h"}) })
	if code != 0 {
		t.Errorf("-h should return 0, got %d", code)
	}
}
