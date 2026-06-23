package canon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var validExts = map[string]bool{".md": true, ".markdown": true}

// SkillFrontmatter is the Agent Skills frontmatter parsed from a SKILL.md.
type SkillFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
}

// PowerFrontmatter is the Kiro POWER.md frontmatter.
type PowerFrontmatter struct {
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"displayName"`
	Description string   `yaml:"description"`
	Keywords    []string `yaml:"keywords"`
	Author      string   `yaml:"author"`
}

// ParseArtifact reads and parses a generic (non-skill, non-power) artifact
// from a Markdown file with YAML frontmatter.
func ParseArtifact(path string) (*Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	fm, body, err := splitFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var meta Metadata
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("%s: invalid frontmatter: %w", path, err)
	}
	return &Artifact{
		Path:           path,
		Dir:            filepath.Dir(path),
		Meta:           meta,
		Body:           strings.TrimRight(body, "\n"),
		RawFrontmatter: fm,
	}, nil
}

// ParseSkill parses an Agent Skills folder (a directory containing SKILL.md).
func ParseSkill(skillDir, skillName string) (*Artifact, error) {
	path := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	fm, body, err := splitFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var sf SkillFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &sf); err != nil {
		return nil, fmt.Errorf("%s: invalid skill frontmatter: %w", path, err)
	}
	meta := Metadata{
		ID:          skillName,
		Kind:        KindSkill,
		Description: sf.Description,
	}
	return &Artifact{
		Path:           path,
		Dir:            skillDir,
		Meta:           meta,
		Body:           strings.TrimRight(body, "\n"),
		RawFrontmatter: fm,
		IsSkill:        true,
	}, nil
}

// ParsePower parses a Kiro power folder (a directory containing POWER.md).
func ParsePower(powerDir, powerName string) (*Artifact, error) {
	path := filepath.Join(powerDir, "POWER.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	fm, body, err := splitFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var pf PowerFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &pf); err != nil {
		return nil, fmt.Errorf("%s: invalid power frontmatter: %w", path, err)
	}
	meta := Metadata{
		ID:          powerName,
		Kind:        KindPower,
		Description: pf.Description,
	}
	return &Artifact{
		Path:           path,
		Dir:            powerDir,
		Meta:           meta,
		Body:           strings.TrimRight(body, "\n"),
		RawFrontmatter: fm,
		IsPower:        true,
	}, nil
}

func splitFrontmatter(s string) (frontmatter, body string, err error) {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return "", "", fmt.Errorf("missing YAML frontmatter (must start with a '---' line)")
	}
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return "", "", fmt.Errorf("missing closing '---' for frontmatter")
	}
	frontmatter = strings.Join(lines[1:closeIdx], "\n")
	body = strings.Join(lines[closeIdx+1:], "\n")
	return frontmatter, body, nil
}

// LoadDir walks root and returns all artifacts: skill folders (SKILL.md),
// power folders (POWER.md), and generic artifact .md files.
func LoadDir(root string) ([]*Artifact, error) {
	var out []*Artifact
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, "POWER.md")); err == nil {
			a, err := ParsePower(path, filepath.Base(path))
			if err != nil {
				return err
			}
			out = append(out, a)
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err == nil {
			a, err := ParseSkill(path, filepath.Base(path))
			if err != nil {
				return err
			}
			out = append(out, a)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if _, err := os.Stat(filepath.Join(path, "POWER.md")); err == nil {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err == nil {
				return filepath.SkipDir
			}
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		if base == "skill.md" || base == "power.md" {
			return nil
		}
		if !validExts[filepath.Ext(path)] {
			return nil
		}
		a, err := ParseArtifact(path)
		if err != nil {
			return err
		}
		out = append(out, a)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
