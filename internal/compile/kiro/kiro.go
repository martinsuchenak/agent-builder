// Package kiro registers the Kiro target compilers. Commands become manual
// steering files (slash-invocable) and rules become always-on steering files,
// both under .kiro/steering/.
package kiro

import (
	"fmt"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/token"
)

func init() {
	compile.Register(canon.KindCommand, "kiro", &CommandCompiler{})
	compile.Register(canon.KindRule, "kiro", &RuleCompiler{})
}

// CommandCompiler renders a command to a Kiro "manual" steering file.
type CommandCompiler struct{}

func (c *CommandCompiler) Compile(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	content := fmt.Sprintf("---\ninclusion: manual\n---\n\n%s\n", body)
	return []compile.Output{{
		RelPath: fmt.Sprintf(".kiro/steering/%s.md", a.Meta.ID),
		Content: []byte(content),
	}}, nil
}

// RuleCompiler renders a rule to a Kiro "always" steering file.
type RuleCompiler struct{}

func (c *RuleCompiler) Compile(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	content := fmt.Sprintf("---\ninclusion: always\n---\n\n%s\n", body)
	return []compile.Output{{
		RelPath: fmt.Sprintf(".kiro/steering/%s.md", a.Meta.ID),
		Content: []byte(content),
	}}, nil
}
