package compile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-builder/internal/canon"
	"agent-builder/internal/token"
)

func init() {
	register := func(target string) {
		Register(canon.KindSkill, target, &SkillCompiler{})
	}
	for _, t := range []string{"claude", "opencode", "codex", "copilot", "kiro"} {
		register(t)
	}
}

// SkillCompiler deploys an Agent Skills folder to each target's skill
// directory, rewriting tokens in SKILL.md and copying supporting files.
type SkillCompiler struct{}

func (c *SkillCompiler) Compile(ctx *Context, a *canon.Artifact) ([]Output, error) {
	if !a.IsSkill {
		return nil, fmt.Errorf("artifact %s is not a skill folder", a.Meta.ID)
	}
	name := a.Meta.ID
	var outputs []Output
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
		dest := filepath.Join(ctx.Spec.SkillDir, name, rel)
		if filepath.Base(path) == "SKILL.md" {
			rendered, err := renderSkillFile(a, ctx)
			if err != nil {
				return err
			}
			outputs = append(outputs, Output{RelPath: dest, Content: rendered})
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		outputs = append(outputs, Output{RelPath: dest, Content: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return outputs, nil
}

func renderSkillFile(a *canon.Artifact, ctx *Context) ([]byte, error) {
	body, err := token.RenderBody(a.Body, ctx.Runtime, ctx.Tools, ctx.Target, ctx.Resolver)
	if err != nil {
		return nil, err
	}
	out := fmt.Sprintf("---\n%s\n---\n\n%[2]s\n", strings.TrimRight(a.RawFrontmatter, "\n"), body)
	return []byte(out), nil
}
