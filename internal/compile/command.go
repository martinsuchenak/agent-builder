package compile

import (
	"fmt"

	"agent-builder/internal/canon"
	"agent-builder/internal/token"
)

func init() {
	register := func(target string) {
		Register(canon.KindCommand, target, &CommandCompiler{})
	}
	register("claude")
	register("opencode")
}

// CommandCompiler renders command artifacts for the markdown slash-command
// targets (claude, opencode).
type CommandCompiler struct{}

func (c *CommandCompiler) Compile(ctx *Context, a *canon.Artifact) ([]Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	var out string
	if ctx.Spec.CommandFrontmatter && a.Meta.Description != "" {
		out = fmt.Sprintf("---\ndescription: %s\n---\n\n%s\n", a.Meta.Description, body)
	} else {
		out = body + "\n"
	}
	return []Output{{
		RelPath: fmt.Sprintf("%s/%s.md", ctx.Spec.CommandDir, a.Meta.ID),
		Content: []byte(out),
	}}, nil
}
