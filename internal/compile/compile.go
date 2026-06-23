// Package compile drives artifact compilation: a registry of Compiler
// implementations keyed by (kind, target), and Execute to run a Plan.
package compile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-builder/internal/canon"
	"agent-builder/internal/token"
)

// Context carries everything a Compiler needs to render one artifact for one target.
type Context struct {
	Target    string
	Tools     *canon.ToolRegistry
	Runtime   token.Runtime
	RulesFile string
	Spec      TargetSpec
	OutRoot   string // absolute path to build/<target>, for read-modify-write compilers
	Resolver  token.Resolver
}

// TargetSpec describes a target's output conventions.
type TargetSpec struct {
	Name               string
	CommandDir         string
	CommandFrontmatter bool
	CommandArgBase     int
	SkillDir           string
	RulesFile          string
	AgentDir           string
	PowerDir           string
	InstallRoot        string // default install destination (user-global; ~ expanded)
	InstallStrip       string // path prefix stripped from relpaths on default install
}

// TargetSpecs holds the built-in per-target output conventions.
var TargetSpecs = map[string]TargetSpec{
	"claude": {
		Name: "claude", CommandDir: ".claude/commands", CommandFrontmatter: false,
		CommandArgBase: 0, SkillDir: ".claude/skills", RulesFile: "CLAUDE.md", AgentDir: ".claude/agents",
		InstallRoot: "~/.claude", InstallStrip: ".claude/",
	},
	"opencode": {
		Name: "opencode", CommandDir: ".opencode/commands", CommandFrontmatter: true,
		CommandArgBase: 1, SkillDir: ".opencode/skills", RulesFile: "AGENTS.md", AgentDir: ".opencode/agents",
		InstallRoot: "~/.config/opencode", InstallStrip: ".opencode/",
	},
	"codex": {
		Name: "codex", CommandDir: ".codex/prompts", CommandFrontmatter: true,
		CommandArgBase: 1, SkillDir: ".agents/skills", RulesFile: "AGENTS.md", AgentDir: ".codex/agents",
		InstallRoot: "~/.codex", InstallStrip: ".codex/",
	},
	"copilot": {
		Name: "copilot", CommandDir: ".github/prompts", CommandFrontmatter: true,
		CommandArgBase: 1, SkillDir: ".agents/skills", RulesFile: ".github/copilot-instructions.md", AgentDir: ".github/chatmodes",
		InstallRoot: ".", InstallStrip: "",
	},
	"kiro": {
		Name: "kiro", CommandDir: ".kiro/steering", CommandFrontmatter: false,
		CommandArgBase: 1, SkillDir: ".kiro/skills", RulesFile: "AGENTS.md", AgentDir: "",
		PowerDir: ".kiro/powers", InstallRoot: "~/.kiro", InstallStrip: ".kiro/",
	},
}

// SpecFor returns the TargetSpec for a target name.
func SpecFor(target string) (TargetSpec, bool) {
	s, ok := TargetSpecs[target]
	return s, ok
}

// Output is a single rendered file produced by a Compiler.
type Output struct {
	RelPath string
	Content []byte
}

// Compiler renders one artifact for one target into one or more Outputs.
type Compiler interface {
	Compile(ctx *Context, a *canon.Artifact) ([]Output, error)
}

type key struct {
	kind   canon.Kind
	target string
}

var registry = map[key]Compiler{}

// Register adds a Compiler for a (kind, target) pair, called from init().
func Register(kind canon.Kind, target string, c Compiler) {
	registry[key{kind, target}] = c
}

// Lookup returns the Compiler registered for a (kind, target) pair.
func Lookup(kind canon.Kind, target string) (Compiler, bool) {
	c, ok := registry[key{kind, target}]
	return c, ok
}

// Plan is a compile request.
type Plan struct {
	Source      string
	Out         string
	Targets     []string
	Tools       *canon.ToolRegistry
	Artifacts   []*canon.Artifact
	OnlyKind    canon.Kind
	SkipErrors  bool
	Interactive bool
	In          interface{ Read(p []byte) (int, error) }
	Out2        interface{ Write(p []byte) (int, error) }
}

// Result is the outcome of compiling one artifact for one target.
type Result struct {
	Target  string
	ArtID   string
	RelPath string
	Files   int
	Skipped string
	Err     error
}

// Execute runs plan: for each target and artifact, looks up and runs the
// matching Compiler and writes its outputs under Out. It returns one Result
// per (artifact, target) attempted.
func Execute(plan Plan) []Result {
	sorted := make([]*canon.Artifact, len(plan.Artifacts))
	copy(sorted, plan.Artifacts)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Meta.Kind != sorted[j].Meta.Kind {
			return sorted[i].Meta.Kind < sorted[j].Meta.Kind
		}
		if sorted[i].Meta.Order != sorted[j].Meta.Order {
			return sorted[i].Meta.Order < sorted[j].Meta.Order
		}
		return sorted[i].Meta.ID < sorted[j].Meta.ID
	})

	var resolver token.Resolver
	if plan.Interactive {
		resolver = newInteractiveResolver(plan.Tools, plan.In, plan.Out2)
	}

	var results []Result
	for _, target := range plan.Targets {
		for _, a := range sorted {
			if plan.OnlyKind != "" && a.Meta.Kind != plan.OnlyKind {
				continue
			}
			res := Result{Target: target, ArtID: a.Meta.ID}
			c, ok := Lookup(a.Meta.Kind, target)
			if !ok {
				res.Skipped = fmt.Sprintf("no compiler for kind=%s target=%s", a.Meta.Kind, target)
				results = append(results, res)
				continue
			}
			spec, _ := SpecFor(target)
			rt, rulesFile := runtimeFor(target, a.Meta.Arguments)
			ctx := &Context{
				Target: target, Tools: plan.Tools, Runtime: rt, RulesFile: rulesFile,
				Spec: spec, OutRoot: filepath.Join(plan.Out, target), Resolver: resolver,
			}
			outs, err := c.Compile(ctx, a)
			if err != nil {
				res.Err = err
				if !plan.SkipErrors {
					results = append(results, res)
					return results
				}
				results = append(results, res)
				continue
			}
			for _, o := range outs {
				full := filepath.Join(plan.Out, target, o.RelPath)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					res.Err = fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)
					results = append(results, res)
					if !plan.SkipErrors {
						return results
					}
					continue
				}
				if err := os.WriteFile(full, o.Content, 0o644); err != nil {
					res.Err = fmt.Errorf("write %s: %w", full, err)
					results = append(results, res)
					if !plan.SkipErrors {
						return results
					}
					continue
				}
				res.RelPath = o.RelPath
				res.Files++
			}
			results = append(results, res)
		}
	}
	return results
}

func runtimeFor(target string, args []canon.Argument) (token.Runtime, string) {
	spec, ok := SpecFor(target)
	if !ok {
		return token.MarkdownRuntime{Args: args, RulesFileVal: "AGENTS.md"}, "AGENTS.md"
	}
	if target == "copilot" {
		return token.CopilotRuntime{Args: args}, spec.RulesFile
	}
	return token.MarkdownRuntime{
		Args:         args,
		ArgIndexBase: spec.CommandArgBase,
		RulesFileVal: spec.RulesFile,
	}, spec.RulesFile
}

type interactiveResolver struct {
	tools *canon.ToolRegistry
	in    io.Reader
	out   io.Writer
}

func newInteractiveResolver(tools *canon.ToolRegistry, in io.Reader, out io.Writer) *interactiveResolver {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stderr
	}
	return &interactiveResolver{tools: tools, in: in, out: out}
}

func (r *interactiveResolver) Resolve(toolID, target string) {
	fmt.Fprintf(r.out, "\nTool %q has no @server declared. Which server does it belong to?\n  (blank = skip and mark unreachable for this run) > ", toolID)
	scanner := bufio.NewScanner(r.in)
	if !scanner.Scan() {
		return
	}
	server := strings.TrimSpace(scanner.Text())
	if server == "" {
		return
	}
	r.tools.SetServer(toolID, server)
	fmt.Fprintf(r.out, "  (tip: persist this by writing {{tool %s@%s ...}} in the source)\n", toolID, server)
}
