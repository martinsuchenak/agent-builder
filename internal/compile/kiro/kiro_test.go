package kiro_test

import (
	"path/filepath"
	"strings"
	"testing"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/compile/kiro"
	"agent-builder/internal/token"
)

func TestCommandCompiler(t *testing.T) {
	a := &canon.Artifact{Meta: canon.Metadata{ID: "do", Kind: canon.KindCommand, Description: "d"}, Body: "do {{input}}"}
	spec, _ := compile.SpecFor("kiro")
	ctx := &compile.Context{Target: "kiro", Tools: canon.NewToolRegistry(), Runtime: token.MarkdownRuntime{RulesFileVal: "AGENTS.md"}, Spec: spec}
	outs, err := (&kiro.CommandCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if outs[0].RelPath != filepath.Join(".kiro/steering/do.md") {
		t.Fatalf("path wrong: %s", outs[0].RelPath)
	}
	if !strings.Contains(string(outs[0].Content), "inclusion: manual") {
		t.Errorf("command should be manual steering:\n%s", outs[0].Content)
	}
}

func TestRuleCompiler(t *testing.T) {
	a := &canon.Artifact{Meta: canon.Metadata{ID: "conv", Kind: canon.KindRule, Description: "d"}, Body: "follow {{rules_file}}"}
	spec, _ := compile.SpecFor("kiro")
	ctx := &compile.Context{Target: "kiro", Tools: canon.NewToolRegistry(), Runtime: token.MarkdownRuntime{RulesFileVal: "AGENTS.md"}, Spec: spec}
	outs, err := (&kiro.RuleCompiler{}).Compile(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	s := string(outs[0].Content)
	if !strings.Contains(s, "inclusion: always") || !strings.Contains(s, "follow AGENTS.md") {
		t.Errorf("rule steering wrong:\n%s", s)
	}
}
