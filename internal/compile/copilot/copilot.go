// Package copilot registers the GitHub Copilot target compilers: commands
// become .github/prompts/<id>.prompt.md (with ${input:} variables) and agents
// become .github/agents/<id>.md.
package copilot

import (
	"fmt"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/token"
)

func init() {
	compile.Register(canon.KindCommand, "copilot", &PromptCommandCompiler{})
	compile.Register(canon.KindAgent, "copilot", &AgentCompiler{})
}

// PromptCommandCompiler renders a command to a Copilot prompt file.
type PromptCommandCompiler struct{}

func (c *PromptCommandCompiler) Compile(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	desc := a.Meta.Description
	content := fmt.Sprintf("---\ndescription: %s\n---\n\n%s\n", desc, body)
	return []compile.Output{{
		RelPath: fmt.Sprintf("%s/%s.prompt.md", ctx.Spec.CommandDir, a.Meta.ID),
		Content: []byte(content),
	}}, nil
}

// AgentCompiler renders an agent to a Copilot custom agent file.
type AgentCompiler struct{}

func (c *AgentCompiler) Compile(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s\n", a.Meta.ID, a.Meta.Description, body)
	return []compile.Output{{
		RelPath: fmt.Sprintf(".github/agents/%s.md", a.Meta.ID),
		Content: []byte(content),
	}}, nil
}
