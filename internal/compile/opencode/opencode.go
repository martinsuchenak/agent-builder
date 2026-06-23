// Package opencode registers the opencode target compiler for agents.
package opencode

import (
	"fmt"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/token"

	"gopkg.in/yaml.v3"
)

func init() {
	compile.Register(canon.KindAgent, "opencode", &AgentCompiler{})
}

// AgentCompiler renders an agent to opencode's .opencode/agents/<id>.md,
// projecting the canonical frontmatter onto opencode's schema.
type AgentCompiler struct{}

func (c *AgentCompiler) Compile(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	fm := buildFrontmatter(a)
	content := fmt.Sprintf("---\n%s---\n\n%s\n", fm, body)
	return []compile.Output{{
		RelPath: fmt.Sprintf("%s/%s.md", ctx.Spec.AgentDir, a.Meta.ID),
		Content: []byte(content),
	}}, nil
}

func buildFrontmatter(a *canon.Artifact) string {
	m := map[string]any{}
	m["description"] = a.Meta.Description
	mode := a.Meta.Mode
	if mode == "" {
		mode = "subagent"
	}
	m["mode"] = mode
	if a.Meta.Model != "" {
		m["model"] = a.Meta.Model
	}
	if a.Meta.Temperature != 0 {
		m["temperature"] = a.Meta.Temperature
	}
	if len(a.Meta.Permissions) > 0 {
		m["permission"] = a.Meta.Permissions
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Sprintf("description: %s\nmode: %s\n", a.Meta.Description, mode)
	}
	return string(out)
}
