package compile

import (
	"fmt"
	"os"
	"path/filepath"

	"agent-builder/internal/canon"
	"agent-builder/internal/token"
)

func init() {
	register := func(target string) {
		Register(canon.KindRule, target, &MergeRuleCompiler{})
	}
	register("claude")
	register("opencode")
	register("codex")
	Register(canon.KindRule, "copilot", &CopilotRuleCompiler{})
}

// MergeRuleCompiler merges a rule into the target's always-on rules file
// (CLAUDE.md / AGENTS.md) as an idempotent managed region. Used by claude,
// opencode, and codex.
type MergeRuleCompiler struct{}

func (c *MergeRuleCompiler) Compile(ctx *Context, a *canon.Artifact) ([]Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	rel := ctx.Spec.RulesFile
	existing := ReadExisting(ctx.OutRoot, rel)
	merged := MergeManaged(existing, "rule", a.Meta.ID, body)
	return []Output{{RelPath: rel, Content: merged}}, nil
}

// CopilotRuleCompiler emits a rule as a standalone
// .github/instructions/<id>.instructions.md file for Copilot.
type CopilotRuleCompiler struct{}

func (c *CopilotRuleCompiler) Compile(ctx *Context, a *canon.Artifact) ([]Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	content := fmt.Sprintf("---\ndescription: %s\napplyTo: \"**\"\n---\n\n%s\n", a.Meta.Description, body)
	return []Output{{
		RelPath: fmt.Sprintf(".github/instructions/%s.instructions.md", a.Meta.ID),
		Content: []byte(content),
	}}, nil
}

// ReadExisting reads an existing managed file under outRoot, returning nil if
// it does not exist (for read-modify-write compilers).
func ReadExisting(outRoot, rel string) []byte {
	if outRoot == "" {
		return nil
	}
	full := filepath.Join(outRoot, rel)
	data, err := os.ReadFile(full)
	if err != nil {
		return nil
	}
	return data
}
