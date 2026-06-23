// Package canon parses canonical agent-builder artifacts (commands, skills,
// rules, agents, powers) and resolves per-target tool invocations.
package canon

// Kind identifies the type of an artifact.
type Kind string

const (
	KindCommand Kind = "command"
	KindSkill   Kind = "skill"
	KindRule    Kind = "rule"
	KindAgent   Kind = "agent"
	KindPower   Kind = "power"
)

// Valid reports whether k is a known artifact kind.
func (k Kind) Valid() bool {
	switch k {
	case KindCommand, KindSkill, KindRule, KindAgent, KindPower:
		return true
	}
	return false
}

// Argument is a declared positional argument of a command or agent.
type Argument struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
}

// Metadata is the parsed frontmatter of a generic (non-skill, non-power) artifact.
type Metadata struct {
	ID          string         `yaml:"id"`
	Kind        Kind           `yaml:"kind"`
	Description string         `yaml:"description"`
	Arguments   []Argument     `yaml:"arguments"`
	Order       int            `yaml:"order"`
	Merge       string         `yaml:"merge"`
	Mode        string         `yaml:"mode"`
	Model       string         `yaml:"model"`
	Temperature float64        `yaml:"temperature"`
	Permissions map[string]any `yaml:"permissions"`
}

// Artifact is a parsed canonical artifact.
type Artifact struct {
	Path           string
	Dir            string // for skills/powers, the folder containing SKILL.md/POWER.md
	Meta           Metadata
	Body           string
	RawFrontmatter string // preserved verbatim for skills/powers
	IsSkill        bool
	IsPower        bool
}

// ArgPosition returns the 0-based position of a declared argument, if present.
func (a *Artifact) ArgPosition(name string) (int, bool) {
	for i, arg := range a.Meta.Arguments {
		if arg.Name == name {
			return i, true
		}
	}
	return -1, false
}
