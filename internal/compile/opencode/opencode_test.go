package opencode_test

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/compile/opencode"
	"agent-builder/internal/token"
)

func TestAgentCompiler(t *testing.T) {
	a := &canon.Artifact{
		Meta: canon.Metadata{
			ID: "reviewer", Kind: canon.KindAgent, Description: "reviews code",
			Model: "sonnet", Temperature: 0.2, Mode: "subagent",
			Permissions: map[string]any{"edit": "deny"},
		},
		Body: "follow {{rules_file}}",
	}
	spec, _ := compile.SpecFor("opencode")
	ctx := &compile.Context{
		Target: "opencode", Tools: canon.NewToolRegistry(),
		Runtime: token.MarkdownRuntime{RulesFileVal: "AGENTS.md"}, RulesFile: "AGENTS.md", Spec: spec,
	}
	outs, err := (&opencode.AgentCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if outs[0].RelPath != filepath.Join(".opencode/agents/reviewer.md") {
		t.Fatalf("path wrong: %s", outs[0].RelPath)
	}
	s := string(outs[0].Content)
	for _, want := range []string{"description: reviews code", "mode: subagent", "model: sonnet", "permission:", "follow AGENTS.md"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}
