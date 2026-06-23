// Package codex registers the Codex target compiler. Codex has no slash-command
// surface, so commands are inlined into AGENTS.md automatically at compile time.
package codex

import (
	"fmt"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/token"
)

func init() {
	compile.Register(canon.KindCommand, "codex", &CommandCompiler{})
}

// CommandCompiler inlines a command into AGENTS.md as a managed region.
type CommandCompiler struct{}

func (c *CommandCompiler) Compile(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	section := fmt.Sprintf("## Command: %s\n\n%s", a.Meta.ID, body)
	existing := compile.ReadExisting(ctx.OutRoot, ctx.Spec.RulesFile)
	merged := compile.MergeManaged(existing, "command", a.Meta.ID, section)
	return []compile.Output{{
		RelPath: ctx.Spec.RulesFile,
		Content: merged,
	}}, nil
}
