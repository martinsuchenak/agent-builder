package power_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/compile/power"
	"agent-builder/internal/token"
)

func makePower(t *testing.T) *canon.Artifact {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "POWER.md"),
		[]byte("---\nname: rel\ndescription: release power\n---\nrelease body"), 0o644)
	os.MkdirAll(filepath.Join(dir, "steering"), 0o755)
	os.WriteFile(filepath.Join(dir, "steering", "x.md"), []byte("extra"), 0o644)
	return &canon.Artifact{
		Dir: dir, Meta: canon.Metadata{ID: "rel", Kind: canon.KindPower, Description: "release power"},
		Body: "release body", RawFrontmatter: "name: rel\ndescription: release power\n", IsPower: true,
	}
}

func TestPowerKiroBundle(t *testing.T) {
	a := makePower(t)
	spec, _ := compile.SpecFor("kiro")
	ctx := &compile.Context{Target: "kiro", Tools: canon.NewToolRegistry(), Runtime: token.MarkdownRuntime{RulesFileVal: "AGENTS.md"}, Spec: spec}
	outs, err := (&power.PowerCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 2 {
		t.Fatalf("expected POWER.md + steering/x.md, got %d", len(outs))
	}
	var skillContent string
	for _, o := range outs {
		if strings.HasSuffix(o.RelPath, "POWER.md") {
			skillContent = string(o.Content)
		}
	}
	if !strings.Contains(skillContent, "release body") {
		t.Errorf("POWER.md body not rendered:\n%s", skillContent)
	}
}

func TestPowerDegradeToSkill(t *testing.T) {
	a := makePower(t)
	spec, _ := compile.SpecFor("claude")
	ctx := &compile.Context{Target: "claude", Tools: canon.NewToolRegistry(), Runtime: token.MarkdownRuntime{RulesFileVal: "CLAUDE.md"}, Spec: spec}
	outs, err := (&power.PowerCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 1 || !strings.HasSuffix(outs[0].RelPath, filepath.Join(".claude/skills/rel/SKILL.md")) {
		t.Fatalf("degrade path wrong: %+v", outs)
	}
	s := string(outs[0].Content)
	if !strings.Contains(s, "name: rel") || !strings.Contains(s, "release body") {
		t.Errorf("degrade skill wrong:\n%s", s)
	}
}

func TestPowerNotAFolder(t *testing.T) {
	a := &canon.Artifact{Meta: canon.Metadata{ID: "x", Kind: canon.KindPower}}
	spec, _ := compile.SpecFor("kiro")
	ctx := &compile.Context{Target: "kiro", Tools: canon.NewToolRegistry(), Runtime: token.MarkdownRuntime{}, Spec: spec}
	if _, err := (&power.PowerCompiler{}).Compile(ctx, a); err == nil {
		t.Error("expected error for non-power artifact")
	}
}
