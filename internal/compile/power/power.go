// Package power registers the power compiler. On Kiro it emits the full
// .kiro/powers/<name>/ bundle (POWER.md + mcp.json + steering/); on other
// targets it degrades the power to a skill.
package power

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	"agent-builder/internal/token"
)

var allTargets = []string{"claude", "opencode", "codex", "copilot", "kiro"}

func init() {
	for _, t := range allTargets {
		compile.Register(canon.KindPower, t, &PowerCompiler{})
	}
}

// PowerCompiler renders a power: a full bundle on Kiro, a skill degrade elsewhere.
type PowerCompiler struct{}

func (c *PowerCompiler) Compile(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	if !a.IsPower {
		return nil, fmt.Errorf("artifact %s is not a power folder", a.Meta.ID)
	}
	if ctx.Target == "kiro" {
		return compileKiroPower(ctx, a)
	}
	return degradeToSkill(ctx, a)
}

func compileKiroPower(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	name := a.Meta.ID
	var outputs []compile.Output
	err := filepath.WalkDir(a.Dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(a.Dir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(ctx.Spec.PowerDir, name, rel)
		if filepath.Base(path) == "POWER.md" {
			rendered, err := renderEntry(a, ctx, "POWER.md")
			if err != nil {
				return err
			}
			outputs = append(outputs, compile.Output{RelPath: dest, Content: rendered})
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		outputs = append(outputs, compile.Output{RelPath: dest, Content: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return outputs, nil
}

func degradeToSkill(ctx *compile.Context, a *canon.Artifact) ([]compile.Output, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	skillFM := fmt.Sprintf("name: %s\ndescription: %s\n", a.Meta.ID, a.Meta.Description)
	content := fmt.Sprintf("---\n%s---\n\n%s\n", skillFM, body)
	return []compile.Output{{
		RelPath: fmt.Sprintf("%s/%s/SKILL.md", ctx.Spec.SkillDir, a.Meta.ID),
		Content: []byte(content),
	}}, nil
}

func renderEntry(a *canon.Artifact, ctx *compile.Context, entryName string) ([]byte, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	out := fmt.Sprintf("---\n%s\n---\n\n%[2]s\n", strings.TrimRight(a.RawFrontmatter, "\n"), body)
	return []byte(out), nil
}
