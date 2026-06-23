package codex_test

import (
	"strings"
	"testing"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/compile/codex"
	"agent-builder/internal/token"
)

func TestCommandCompilerInline(t *testing.T) {
	out := t.TempDir()
	a := &canon.Artifact{
		Meta: canon.Metadata{ID: "ping", Kind: canon.KindCommand},
		Body: "do {{input}}",
	}
	spec, _ := compile.SpecFor("codex")
	ctx := &compile.Context{
		Target: "codex", Tools: canon.NewToolRegistry(),
		Runtime: token.MarkdownRuntime{RulesFileVal: "AGENTS.md"}, RulesFile: "AGENTS.md", Spec: spec, OutRoot: out,
	}
	outs, err := (&codex.CommandCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if outs[0].RelPath != "AGENTS.md" {
		t.Fatalf("expected AGENTS.md, got %s", outs[0].RelPath)
	}
	if !strings.Contains(string(outs[0].Content), "Command: ping") || !strings.Contains(string(outs[0].Content), "BEGIN ab:command ping") {
		t.Errorf("inline merge wrong:\n%s", outs[0].Content)
	}
}
