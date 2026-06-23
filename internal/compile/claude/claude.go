// Package claude registers the Claude Code target compiler for agents.
package claude

import (
	"fmt"
	"sort"
	"strings"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/token"
)

func init() {
	compile.Register(canon.KindAgent, "claude", &AgentCompiler{})
}

// AgentCompiler renders an agent to Claude Code's .claude/agents/<id>.md,
// projecting deny-permissions onto disallowedTools.
type AgentCompiler struct{}

func (c *AgentCompiler) Compile(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	content := fmt.Sprintf("---\n%s---\n\n%s\n", buildAgentFrontmatter(a), body)
	return []compile.Output{{
		RelPath: fmt.Sprintf("%s/%s.md", ctx.Spec.AgentDir, a.Meta.ID),
		Content: []byte(content),
	}}, nil
}

func buildAgentFrontmatter(a *canon.Artifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", a.Meta.ID)
	fmt.Fprintf(&b, "description: %s\n", a.Meta.Description)
	if a.Meta.Model != "" {
		fmt.Fprintf(&b, "model: %s\n", a.Meta.Model)
	}
	if dt := deniedTools(a); dt != "" {
		fmt.Fprintf(&b, "disallowedTools: %s\n", dt)
	}
	return b.String()
}

var permToTools = map[string][]string{
	"edit":     {"Write", "Edit"},
	"bash":     {"Bash"},
	"read":     {"Read"},
	"webfetch": {"WebFetch"},
}

func deniedTools(a *canon.Artifact) string {
	set := map[string]bool{}
	for perm, val := range a.Meta.Permissions {
		s, ok := val.(string)
		if !ok || s != "deny" {
			continue
		}
		for _, t := range permToTools[perm] {
			set[t] = true
		}
	}
	var tools []string
	for t := range set {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	return strings.Join(tools, ", ")
}
