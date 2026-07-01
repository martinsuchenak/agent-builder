package compile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	_ "agent-builder/internal/compile/kiro"
	_ "agent-builder/internal/compile/power"
	"agent-builder/internal/token"
)

func TestDerivedInvocationFromServer(t *testing.T) {
	reg := canon.NewToolRegistry()

	art := &canon.Artifact{
		Meta: canon.Metadata{ID: "ping", Kind: canon.KindCommand},
		Body: "Run {{tool echo@fortix msg=input}} now.",
	}

	plan := compile.Plan{
		Out:       filepath.Join(t.TempDir(), "build"),
		Targets:   []string{"opencode"},
		Tools:     reg,
		Artifacts: []*canon.Artifact{art},
	}
	results := compile.Execute(plan)
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("expected 1 successful result, got %#v", results)
	}
	if got := reg.Server("echo"); got != "fortix" {
		t.Fatalf("server not recorded from @server token; got %q", got)
	}
	out, err := os.ReadFile(filepath.Join(plan.Out, "opencode", ".opencode", "commands", "ping.md"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	want := "call the `llmrouter_fortix__echo` tool with msg=$ARGUMENTS"
	if !strings.Contains(string(out), want) {
		t.Fatalf("derived invocation mismatch; want substring %q, got:\n%s", want, out)
	}
}

func TestInteractiveResolveByServer(t *testing.T) {
	reg := canon.NewToolRegistry()
	art := &canon.Artifact{
		Meta: canon.Metadata{ID: "ping", Kind: canon.KindCommand},
		Body: "Run {{tool echo msg=input}} now.",
	}
	plan := compile.Plan{
		Out:         filepath.Join(t.TempDir(), "build"),
		Targets:     []string{"opencode", "claude"},
		Tools:       reg,
		Artifacts:   []*canon.Artifact{art},
		Interactive: true,
		In:          strings.NewReader("fortix\n"),
		PromptOut:   &strings.Builder{},
	}
	results := compile.Execute(plan)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %#v", len(results), results)
	}
	for _, r := range results {
		if r.Err != nil {
			t.Fatalf("unexpected error for %s/%s: %v", r.Target, r.ArtID, r.Err)
		}
	}
	if got := reg.Server("echo"); got != "fortix" {
		t.Fatalf("server not persisted via prompt; got %q", got)
	}
}

func TestPowerKiroBundleAndSkillDegrade(t *testing.T) {
	powerDir := filepath.Join(t.TempDir(), "release-workflow")
	powerBody := "---\nname: release-workflow\ndescription: Releases.\n---\n\nDo the release.\n"
	if err := os.MkdirAll(powerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(powerDir, "POWER.md"), []byte(powerBody), 0o644); err != nil {
		t.Fatal(err)
	}
	art := &canon.Artifact{
		Dir:            powerDir,
		Meta:           canon.Metadata{ID: "release-workflow", Kind: canon.KindPower, Description: "Releases."},
		Body:           "Do the release.",
		RawFrontmatter: "name: release-workflow\ndescription: Releases.\n",
		IsPower:        true,
	}
	reg := canon.NewToolRegistry()

	for _, tc := range []struct {
		target  string
		wantRel string
	}{
		{"kiro", ".kiro/powers/release-workflow/POWER.md"},
		{"claude", ".claude/skills/release-workflow/SKILL.md"},
	} {
		plan := compile.Plan{
			Out: filepath.Join(t.TempDir(), "build"), Targets: []string{tc.target},
			Tools: reg, Artifacts: []*canon.Artifact{art},
		}
		res := compile.Execute(plan)
		if len(res) != 1 || res[0].Err != nil {
			t.Fatalf("%s: expected success, got %#v", tc.target, res)
		}
		full := filepath.Join(plan.Out, tc.target, tc.wantRel)
		data, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("%s: read %s: %v", tc.target, full, err)
		}
		if !strings.Contains(string(data), "Do the release.") {
			t.Fatalf("%s: body missing in %s", tc.target, tc.wantRel)
		}
	}
}

func TestMergeManagedReplacesExistingBlock(t *testing.T) {
	first := compile.MergeManaged(nil, "rule", "a", "first body")
	second := compile.MergeManaged(first, "rule", "b", "second body")
	if !strings.Contains(string(second), "first body") || !strings.Contains(string(second), "second body") {
		t.Fatalf("expected both blocks present, got:\n%s", second)
	}
	replaced := compile.MergeManaged(second, "rule", "a", "updated body")
	if strings.Contains(string(replaced), "first body") {
		t.Fatalf("old body should have been replaced, got:\n%s", replaced)
	}
	if !strings.Contains(string(replaced), "updated body") || !strings.Contains(string(replaced), "second body") {
		t.Fatalf("expected updated + second body, got:\n%s", replaced)
	}
}

func TestSkillCompiler(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: sk\ndescription: a skill\n---\nskill body"), 0o644)
	os.MkdirAll(filepath.Join(dir, "references"), 0o755)
	os.WriteFile(filepath.Join(dir, "references", "r.md"), []byte("ref content"), 0o644)
	a := &canon.Artifact{
		Dir: dir, Meta: canon.Metadata{ID: "sk", Kind: canon.KindSkill, Description: "a skill"},
		Body: "skill body", RawFrontmatter: "name: sk\ndescription: a skill\n", IsSkill: true,
	}
	spec, _ := compile.SpecFor("claude")
	ctx := &compile.Context{Target: "claude", Tools: canon.NewToolRegistry(), Runtime: token.MarkdownRuntime{RulesFileVal: "CLAUDE.md"}, Spec: spec}
	outs, err := (&compile.SkillCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 2 {
		t.Fatalf("expected SKILL.md + references/r.md, got %d", len(outs))
	}
	var skillText string
	for _, o := range outs {
		if strings.HasSuffix(o.RelPath, "SKILL.md") {
			skillText = string(o.Content)
		}
	}
	if !strings.Contains(skillText, "name: sk") || !strings.Contains(skillText, "skill body") {
		t.Errorf("SKILL.md not rendered correctly:\n%s", skillText)
	}
}

func TestMergeRuleCompilerAccumulates(t *testing.T) {
	out := t.TempDir()
	spec, _ := compile.SpecFor("claude")
	ctx := &compile.Context{Target: "claude", Tools: canon.NewToolRegistry(), Runtime: token.MarkdownRuntime{RulesFileVal: "CLAUDE.md"}, RulesFile: "CLAUDE.md", Spec: spec, OutRoot: out}
	c := &compile.MergeRuleCompiler{}
	a1 := &canon.Artifact{Meta: canon.Metadata{ID: "r1", Kind: canon.KindRule}, Body: "rule one"}
	a2 := &canon.Artifact{Meta: canon.Metadata{ID: "r2", Kind: canon.KindRule}, Body: "rule two"}

	o1, err := c.Compile(ctx, a1)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(out, "CLAUDE.md"), o1[0].Content, 0o644)

	o2, err := c.Compile(ctx, a2)
	if err != nil {
		t.Fatal(err)
	}
	merged := string(o2[0].Content)
	if !strings.Contains(merged, "rule one") || !strings.Contains(merged, "rule two") {
		t.Errorf("second merge should contain both rules:\n%s", merged)
	}

	os.WriteFile(filepath.Join(out, "CLAUDE.md"), o2[0].Content, 0o644)
	o1b, _ := c.Compile(ctx, a1)
	if strings.Count(string(o1b[0].Content), "rule one") != 1 {
		t.Errorf("re-merge should be idempotent:\n%s", o1b[0].Content)
	}
}

func TestCopilotRuleCompiler(t *testing.T) {
	a := &canon.Artifact{Meta: canon.Metadata{ID: "conv", Kind: canon.KindRule, Description: "conventions"}, Body: "be good"}
	spec, _ := compile.SpecFor("copilot")
	ctx := &compile.Context{Target: "copilot", Tools: canon.NewToolRegistry(), Runtime: token.CopilotRuntime{}, Spec: spec}
	outs, err := (&compile.CopilotRuleCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(outs[0].RelPath, ".github/instructions/conv.instructions.md") {
		t.Fatalf("path wrong: %s", outs[0].RelPath)
	}
	s := string(outs[0].Content)
	if !strings.Contains(s, "description: conventions") || !strings.Contains(s, "be good") {
		t.Errorf("copilot rule wrong:\n%s", s)
	}
}

func TestMergeManagedRegionsPreservesOutsideContent(t *testing.T) {
	compiled := []byte("<!-- BEGIN ab:rule a -->\nA\n<!-- END ab:rule a -->\n" +
		"<!-- BEGIN ab:command b -->\nB\n<!-- END ab:command b -->\n")
	dest := []byte("hand intro\n\n<!-- BEGIN ab:rule a -->\nOLD A\n<!-- END ab:rule a -->\n")
	out := compile.MergeManagedRegions(dest, compiled)
	s := string(out)
	if !strings.Contains(s, "hand intro") {
		t.Errorf("outside content lost:\n%s", s)
	}
	if strings.Contains(s, "OLD A") || !strings.Contains(s, "A") {
		t.Errorf("rule a not replaced:\n%s", s)
	}
	if !strings.Contains(s, "B") {
		t.Errorf("command b not added:\n%s", s)
	}
}

func TestExecuteSkipErrorsAndOnlyKind(t *testing.T) {
	reg := canon.NewToolRegistry()
	good := &canon.Artifact{Meta: canon.Metadata{ID: "good", Kind: canon.KindCommand}, Body: "plain"}
	bad := &canon.Artifact{Meta: canon.Metadata{ID: "bad", Kind: canon.KindCommand}, Body: "{{tool mystery x=1}}"}

	res := compile.Execute(compile.Plan{
		Out: filepath.Join(t.TempDir(), "b"), Targets: []string{"claude"},
		Tools: reg, Artifacts: []*canon.Artifact{bad, good}, SkipErrors: true,
	})
	var errs, oks int
	for _, r := range res {
		if r.Err != nil {
			errs++
		} else if r.Skipped == "" {
			oks++
		}
	}
	if errs == 0 || oks == 0 {
		t.Fatalf("SkipErrors should yield an error + an ok, got errs=%d oks=%d", errs, oks)
	}

	res2 := compile.Execute(compile.Plan{
		Out: filepath.Join(t.TempDir(), "b2"), Targets: []string{"claude"},
		Tools: reg, Artifacts: []*canon.Artifact{good}, OnlyKind: canon.KindRule,
	})
	if len(res2) != 0 {
		t.Fatalf("OnlyKind=rule should filter out the command, got %+v", res2)
	}
}

func TestExecuteSkippedCompilerAndInteractiveBlank(t *testing.T) {
	reg := canon.NewToolRegistry()
	agent := &canon.Artifact{Meta: canon.Metadata{ID: "ag", Kind: canon.KindAgent}, Body: "x"}
	res := compile.Execute(compile.Plan{
		Out: filepath.Join(t.TempDir(), "b"), Targets: []string{"kiro"},
		Tools: reg, Artifacts: []*canon.Artifact{agent},
	})
	if len(res) != 1 || res[0].Skipped == "" {
		t.Fatalf("expected a skip for agent/kiro (no compiler), got %+v", res)
	}

	bad := &canon.Artifact{Meta: canon.Metadata{ID: "bad", Kind: canon.KindCommand}, Body: "{{tool mystery x=1}}"}
	res2 := compile.Execute(compile.Plan{
		Out: filepath.Join(t.TempDir(), "b2"), Targets: []string{"claude"},
		Tools: reg, Artifacts: []*canon.Artifact{bad}, SkipErrors: true,
		Interactive: true, In: strings.NewReader("\n"), PromptOut: &strings.Builder{},
	})
	if len(res2) != 1 || res2[0].Err == nil {
		t.Fatalf("blank interactive input should leave tool undefined (error), got %+v", res2)
	}
}
