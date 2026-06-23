package canon

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKindValid(t *testing.T) {
	valid := []Kind{KindCommand, KindSkill, KindRule, KindAgent, KindPower}
	for _, k := range valid {
		if !k.Valid() {
			t.Errorf("%q should be valid", k)
		}
	}
	if Kind("nope").Valid() {
		t.Error("invalid kind reported valid")
	}
}

func TestArtifactArgPosition(t *testing.T) {
	a := &Artifact{}
	a.Meta.Arguments = []Argument{{Name: "x"}, {Name: "y"}}
	pos, ok := a.ArgPosition("y")
	if !ok || pos != 1 {
		t.Errorf("ArgPosition wrong: %d %v", pos, ok)
	}
	if _, ok := a.ArgPosition("z"); ok {
		t.Error("unknown arg should not be found")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSplitFrontmatter(t *testing.T) {
	if _, _, err := splitFrontmatter("no frontmatter here"); err == nil {
		t.Error("expected error for missing frontmatter")
	}
	if _, _, err := splitFrontmatter("---\nid: x\nkind: command"); err == nil {
		t.Error("expected error for missing closing delimiter")
	}
	fm, body, err := splitFrontmatter("---\nid: x\n---\nhello")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(fm, "id: x") || body != "hello" {
		t.Errorf("split wrong: fm=%q body=%q", fm, body)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestParseArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cmd.md")
	write(t, path, "---\nid: cmd\nkind: command\ndescription: d\n---\nbody text")
	a, err := ParseArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.Meta.ID != "cmd" || a.Meta.Kind != KindCommand {
		t.Errorf("meta wrong: %+v", a.Meta)
	}
	if a.Body != "body text" {
		t.Errorf("body wrong: %q", a.Body)
	}
	if !contains(a.RawFrontmatter, "kind: command") {
		t.Errorf("raw frontmatter not kept: %q", a.RawFrontmatter)
	}

	if _, err := ParseArtifact(filepath.Join(t.TempDir(), "missing.md")); err == nil {
		t.Error("expected error for missing file")
	}

	bad := filepath.Join(t.TempDir(), "bad.md")
	write(t, bad, "---\nid: x\n  bad: : yaml\n---\nbody")
	if _, err := ParseArtifact(bad); err == nil {
		t.Error("expected error for invalid yaml frontmatter")
	}
}

func TestParseSkillAndPower(t *testing.T) {
	dir := t.TempDir()
	sk := filepath.Join(dir, "myskill")
	write(t, filepath.Join(sk, "SKILL.md"), "---\nname: myskill\ndescription: a skill\n---\nskill body")
	a, err := ParseSkill(sk, "myskill")
	if err != nil {
		t.Fatal(err)
	}
	if !a.IsSkill || a.Meta.ID != "myskill" || a.Meta.Kind != KindSkill || a.Meta.Description != "a skill" {
		t.Errorf("skill parse wrong: %+v", a.Meta)
	}

	pw := filepath.Join(dir, "mypower")
	write(t, filepath.Join(pw, "POWER.md"), "---\nname: mypower\ndescription: a power\n---\npower body")
	p, err := ParsePower(pw, "mypower")
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsPower || p.Meta.ID != "mypower" || p.Meta.Kind != KindPower {
		t.Errorf("power parse wrong: %+v", p.Meta)
	}

	if _, err := ParseSkill(filepath.Join(dir, "nope"), "nope"); err == nil {
		t.Error("expected error for missing SKILL.md")
	}
	if _, err := ParsePower(filepath.Join(dir, "nope"), "nope"); err == nil {
		t.Error("expected error for missing POWER.md")
	}
}

func TestLoadDir(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "commands", "foo.md"), "---\nid: foo\nkind: command\ntargets: [claude]\n---\nfoo body")
	write(t, filepath.Join(root, "skills", "sk", "SKILL.md"), "---\nname: sk\ndescription: s\n---\nsk body")
	write(t, filepath.Join(root, "powers", "pw", "POWER.md"), "---\nname: pw\ndescription: p\n---\npw body")
	write(t, filepath.Join(root, "skills", "sk", "references", "ref.md"), "ref content")

	arts, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[Kind]bool{}
	for _, a := range arts {
		kinds[a.Meta.Kind] = true
	}
	if !kinds[KindCommand] || !kinds[KindSkill] || !kinds[KindPower] {
		t.Errorf("LoadDir missed kinds: %+v", kinds)
	}
	if len(arts) != 3 {
		t.Errorf("expected 3 artifacts (ref.md should not be parsed), got %d", len(arts))
	}
}

func TestDefaultInvocationAndNewRegistry(t *testing.T) {
	reg := NewToolRegistry()
	for _, tgt := range []string{"claude", "opencode", "codex", "copilot", "kiro"} {
		if _, ok := reg.Invocation[tgt]; !ok {
			t.Errorf("default invocation missing %s", tgt)
		}
	}
}

func TestLoadTargets(t *testing.T) {
	reg, err := LoadTargets(filepath.Join(t.TempDir(), "targets.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if reg.Invocation["claude"].Default == "" {
		t.Error("defaults should apply when file absent")
	}

	path := filepath.Join(t.TempDir(), "targets.yaml")
	write(t, path, "invocation:\n  claude: 'custom claude'\n  opencode:\n    default: 'oc default'\n    scriptling: 'oc scriptling'\n")
	reg2, err := LoadTargets(path)
	if err != nil {
		t.Fatal(err)
	}
	if reg2.Invocation["claude"].Default != "custom claude" {
		t.Errorf("claude override not applied: %+v", reg2.Invocation["claude"])
	}
	if reg2.Invocation["opencode"].TemplateFor("scriptling") != "oc scriptling" {
		t.Error("per-server override not applied")
	}

	bad := filepath.Join(t.TempDir(), "bad.yaml")
	write(t, bad, "invocation:\n  claude: [1, 2]\n")
	if _, err := LoadTargets(bad); err == nil {
		t.Error("expected error for invalid invocation yaml")
	}
}

func TestToolRegistryResolve(t *testing.T) {
	reg := NewToolRegistry()
	reg.SetServer("echo", "fortix")
	reg.SetServer("exec", "llmrouter")

	if tmpl, reach, derived := reg.Resolve("echo", "claude"); reach != Reachable || !derived || tmpl == "" {
		t.Errorf("reachable derived wrong: %q %v %v", tmpl, reach, derived)
	}
	if _, reach, _ := reg.Resolve("echo", "codex"); reach != Unreachable {
		t.Error("codex should be unreachable")
	}
	if _, reach, _ := reg.Resolve("exec", "claude"); reach != Reachable {
		t.Error("root-server llmrouter should be reachable")
	}
	claudeTmpl, _, _ := reg.Resolve("exec", "claude")
	if !contains(claudeTmpl, "mcp__llmrouter__{tool}") {
		t.Errorf("root-server should use llmrouter override, got %q", claudeTmpl)
	}
	if _, reach, _ := reg.Resolve("echo", "nonexistent-target"); reach != Undefined {
		t.Error("unknown target should be undefined")
	}
	reg.SetServer("novia", "fortix")
	if _, reach, _ := reg.Resolve("mystery", "claude"); reach != Undefined {
		t.Error("tool with no server should be undefined")
	}
}

func TestInvocationSpecYAMLRoundTrip(t *testing.T) {
	original := InvocationSpec{Default: "d", ByServer: map[string]string{"scriptling": "s"}}
	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var back InvocationSpec
	if err := yaml.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Default != "d" || back.ByServer["scriptling"] != "s" {
		t.Errorf("round-trip lost data: %+v", back)
	}

	var scalar InvocationSpec
	if err := yaml.Unmarshal([]byte("just-a-string"), &scalar); err != nil {
		t.Fatal(err)
	}
	if scalar.Default != "just-a-string" {
		t.Errorf("scalar unmarshal wrong: %+v", scalar)
	}
}

func TestInvocationSpecMarshalScalar(t *testing.T) {
	out, err := yaml.Marshal(InvocationSpec{Default: "unreachable"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "unreachable\n" {
		t.Errorf("scalar-only spec should marshal as scalar, got %q", out)
	}
}
