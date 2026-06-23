package claude_test

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/compile/claude"
	"agent-builder/internal/token"
)

func TestAgentCompiler(t *testing.T) {
	a := &canon.Artifact{
		Meta: canon.Metadata{
			ID:          "reviewer",
			Kind:        canon.KindAgent,
			Description: "reviews code",
			Model:       "sonnet",
			Permissions: map[string]any{"edit": "deny", "bash": "deny", "read": "allow"},
		},
		Body: "follow {{rules_file}}",
	}
	spec, _ := compile.SpecFor("claude")
	ctx := &compile.Context{
		Target: "claude", Tools: canon.NewToolRegistry(),
		Runtime: token.MarkdownRuntime{RulesFileVal: "CLAUDE.md"}, RulesFile: "CLAUDE.md", Spec: spec,
	}
	outs, err := (&claude.AgentCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) != 1 || outs[0].RelPath != filepath.Join(".claude/agents/reviewer.md") {
		t.Fatalf("output path wrong: %+v", outs)
	}
	s := string(outs[0].Content)
	for _, want := range []string{"name: reviewer", "description: reviews code", "model: sonnet", "disallowedTools: Bash, Edit, Write", "follow CLAUDE.md"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}
