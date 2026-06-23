package copilot_test

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/compile/copilot"
	"agent-builder/internal/token"
)

func TestPromptCommandCompiler(t *testing.T) {
	a := &canon.Artifact{
		Meta: canon.Metadata{ID: "do", Kind: canon.KindCommand, Description: "do thing", Arguments: []canon.Argument{{Name: "input"}}},
		Body: "input is {{input}}",
	}
	spec, _ := compile.SpecFor("copilot")
	ctx := &compile.Context{Target: "copilot", Tools: canon.NewToolRegistry(), Runtime: token.CopilotRuntime{}, Spec: spec}
	outs, err := (&copilot.PromptCommandCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if outs[0].RelPath != filepath.Join(".github/prompts/do.prompt.md") {
		t.Fatalf("path wrong: %s", outs[0].RelPath)
	}
	s := string(outs[0].Content)
	if !strings.Contains(s, "description: do thing") || !strings.Contains(s, "input is ${input:input}") {
		t.Errorf("prompt command wrong:\n%s", s)
	}
}

func TestAgentCompiler(t *testing.T) {
	a := &canon.Artifact{Meta: canon.Metadata{ID: "bot", Kind: canon.KindAgent, Description: "a bot"}, Body: "be a bot"}
	spec, _ := compile.SpecFor("copilot")
	ctx := &compile.Context{Target: "copilot", Tools: canon.NewToolRegistry(), Runtime: token.CopilotRuntime{}, Spec: spec}
	outs, err := (&copilot.AgentCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if outs[0].RelPath != filepath.Join(".github/agents/bot.md") {
		t.Fatalf("path wrong: %s", outs[0].RelPath)
	}
	s := string(outs[0].Content)
	if !strings.Contains(s, "name: bot") || !strings.Contains(s, "be a bot") {
		t.Errorf("agent wrong:\n%s", s)
	}
}
