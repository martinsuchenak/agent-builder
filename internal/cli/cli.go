// Package cli parses arguments and dispatches the ab subcommands
// (compile, validate, new, targets).
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-builder/internal/canon"
	"agent-builder/internal/compile"
	_ "agent-builder/internal/compile/claude"
	_ "agent-builder/internal/compile/codex"
	_ "agent-builder/internal/compile/copilot"
	_ "agent-builder/internal/compile/kiro"
	_ "agent-builder/internal/compile/opencode"
	_ "agent-builder/internal/compile/power"
	"agent-builder/internal/token"
)

const usage = `agent-builder — compile canonical AI-agent artifacts to platform-specific formats

Usage:
  agent-builder compile  [source] [out] [flags]   source/out default to ./ab-src and ./build
  agent-builder validate [source] [flags]
  agent-builder new      <kind> <id> [source] [flags]
  agent-builder targets
  agent-builder help

Commands:
  compile   Compile artifacts to one or more targets
  validate  Check canonical artifacts for errors
  new       Scaffold a new canonical artifact (kind: command|skill|rule|agent|power)
  targets   List configured targets
  help      Show this message

Paths (positional or flag):
  source   canonical source root, containing artifacts/ (and optional targets.yaml)
  out      compile output root (compile only)

Compile flags:
  --source <dir>     canonical source root (default ./ab-src)
  --out <dir>        output root (default ./build)
  --target <name>    target to compile (repeatable; default all known)
  --kind <kind>      only compile artifacts of this kind
  --continue         keep going after errors (skip and report)
  --interactive      force interactive prompting (else auto when stdin is a TTY)
  --non-interactive  never prompt; undefined tools error out
`

// Main dispatches a subcommand from args and returns the process exit code.
func Main(args []string) int {
	if len(args) < 1 {
		fmt.Print(usage)
		return 0
	}
	switch args[0] {
	case "compile":
		return runCompile(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "new":
		return runNew(args[1:])
	case "targets":
		return runTargets(args[1:])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

var knownTargets = []string{"claude", "opencode", "codex", "copilot", "kiro"}

func runTargets(args []string) int {
	for _, t := range knownTargets {
		fmt.Println(t)
	}
	return 0
}

func parseFlags(args []string, multi map[string]*[]string, single map[string]*string, bools map[string]*bool) ([]string, error) {
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--"):
			name := strings.TrimPrefix(a, "--")
			if ptr, ok := bools[name]; ok {
				*ptr = true
				continue
			}
			var val string
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				val = name[eq+1:]
				name = name[:eq]
			} else {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("flag --%s requires a value", name)
				}
				i++
				val = args[i]
			}
			if ptr, ok := single[name]; ok {
				*ptr = val
			} else if ptr, ok := multi[name]; ok {
				*ptr = append(*ptr, val)
			} else {
				return nil, fmt.Errorf("unknown flag --%s", name)
			}
		default:
			positional = append(positional, a)
		}
	}
	return positional, nil
}

func runCompile(args []string) int {
	var source, out, kindFlag string
	var targets []string
	var cont, nonInteractive, interactive bool
	pos, err := parseFlags(args,
		map[string]*[]string{"target": &targets},
		map[string]*string{"source": &source, "out": &out, "kind": &kindFlag},
		map[string]*bool{"continue": &cont, "non-interactive": &nonInteractive, "interactive": &interactive},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if source == "" && len(pos) > 0 {
		source = pos[0]
	}
	if source == "" {
		source = "ab-src"
	}
	if out == "" && len(pos) > 1 {
		out = pos[1]
	}
	if out == "" {
		out = "build"
	}
	if len(targets) == 0 {
		targets = knownTargets
	}

	arts, err := canon.LoadDir(filepath.Join(source, "artifacts"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading artifacts:", err)
		return 1
	}
	tools, err := canon.LoadTargets(filepath.Join(source, "targets.yaml"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading targets.yaml:", err)
		return 1
	}

	interactiveMode := interactive || (!nonInteractive && isTerminal(os.Stdin))

	plan := compile.Plan{
		Source:      source,
		Out:         out,
		Targets:     targets,
		Tools:       tools,
		Artifacts:   arts,
		SkipErrors:  cont,
		Interactive: interactiveMode,
	}
	if kindFlag != "" {
		plan.OnlyKind = canon.Kind(kindFlag)
	}

	results := compile.Execute(plan)
	written, skipped, failed := 0, 0, 0
	for _, r := range results {
		switch {
		case r.Err != nil:
			failed++
			fmt.Fprintf(os.Stderr, "  FAIL  [%s] %s: %v\n", r.Target, r.ArtID, r.Err)
		case r.Skipped != "":
			skipped++
			fmt.Fprintf(os.Stderr, "  SKIP  [%s] %s: %s\n", r.Target, r.ArtID, r.Skipped)
		default:
			written++
			if r.Files > 1 {
				fmt.Printf("  OK    [%s] %s -> %s (%d files)\n", r.Target, r.ArtID, filepath.Dir(r.RelPath), r.Files)
			} else {
				fmt.Printf("  OK    [%s] %s -> %s\n", r.Target, r.ArtID, r.RelPath)
			}
		}
	}
	fmt.Printf("\n%d written, %d skipped, %d failed\n", written, skipped, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

func runValidate(args []string) int {
	var source string
	pos, err := parseFlags(args, nil,
		map[string]*string{"source": &source},
		nil,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if source == "" && len(pos) > 0 {
		source = pos[0]
	}
	if source == "" {
		source = "ab-src"
	}

	arts, err := canon.LoadDir(filepath.Join(source, "artifacts"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading artifacts:", err)
		return 1
	}
	tools, err := canon.LoadTargets(filepath.Join(source, "targets.yaml"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading targets.yaml:", err)
		return 1
	}

	var problems []string
	for _, a := range arts {
		base := filepath.Base(a.Path)
		if a.Meta.ID == "" {
			problems = append(problems, fmt.Sprintf("%s: missing frontmatter field 'id'", base))
		}
		if !a.Meta.Kind.Valid() {
			problems = append(problems, fmt.Sprintf("%s: invalid or missing 'kind' (got %q)", base, a.Meta.Kind))
		}
		if perr := validateTokens(a, tools); perr != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", base, perr))
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		fmt.Println("  -", p)
	}
	if len(problems) == 0 {
		fmt.Printf("OK — %d artifact(s) valid\n", len(arts))
		return 0
	}
	fmt.Printf("\n%d problem(s)\n", len(problems))
	return 1
}

func runNew(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: agent-builder new <command|skill|rule|agent|power> <id> [source] [--source dir]")
		return 2
	}
	kind := args[0]
	id := args[1]
	var source string
	pos, err := parseFlags(args[2:], nil, map[string]*string{"source": &source}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 2
	}
	if source == "" && len(pos) > 0 {
		source = pos[0]
	}
	if source == "" {
		source = "ab-src"
	}
	if !canon.Kind(kind).Valid() {
		fmt.Fprintf(os.Stderr, "invalid kind %q\n", kind)
		return 2
	}
	k := canon.Kind(kind)
	baseDir := filepath.Join(source, "artifacts", kind+"s")
	var path, tmpl string
	if k == canon.KindSkill || k == canon.KindPower {
		entry := "SKILL.md"
		if k == canon.KindPower {
			entry = "POWER.md"
		}
		if err := os.MkdirAll(filepath.Join(baseDir, id), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "mkdir:", err)
			return 1
		}
		path = filepath.Join(baseDir, id, entry)
		tmpl = scaffoldFolder(k, id, entry)
	} else {
		if err := os.MkdirAll(baseDir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "mkdir:", err)
			return 1
		}
		path = filepath.Join(baseDir, id+".md")
		tmpl = scaffold(k, id)
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "already exists: %s\n", path)
		return 1
	}
	if err := os.WriteFile(path, []byte(tmpl), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		return 1
	}
	fmt.Println("created", path)
	return 0
}

func scaffoldFolder(k canon.Kind, id, entry string) string {
	if k == canon.KindPower {
		return fmt.Sprintf(`---
name: %s
displayName: %s
description: TODO — what this power does and when to activate it
keywords: []
---
# Onboarding

TODO: dependency/setup checks.

# Steering

TODO: workflows and best practices.
`, id, id)
	}
	return fmt.Sprintf(`---
name: %s
description: TODO — what this skill does and when to use it
---

TODO: skill instructions.
`, id)
}

func scaffold(k canon.Kind, id string) string {
	return fmt.Sprintf(`---
id: %s
kind: %s
description: TODO — what this artifact does and when to use it
arguments: []
---

TODO: body. Use tokens:
  {{input}}         all arguments
  {{arg:NAME}}      a declared argument
  {{rules_file}}    target's rules file
  {{tool ID@server k=v}}  tool call resolved per target
`, id, k)
}

func validateTokens(a *canon.Artifact, tools *canon.ToolRegistry) error {
	for _, t := range knownTargets {
		if err := token.ValidateBody(a.Body, tools, t); err != nil {
			return err
		}
	}
	return nil
}

func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
