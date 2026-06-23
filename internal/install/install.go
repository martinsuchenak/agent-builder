// Package install copies compiled build output into each target platform's
// config location, prompting before overwriting existing files.
package install

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agent-builder/internal/compile"
)

var errAbort = errors.New("install aborted")

// Action describes what happened to a file during install.
type Action int

const (
	Created Action = iota
	Overwritten
	Skipped
	Merged
)

func (a Action) String() string {
	switch a {
	case Created:
		return "created"
	case Overwritten:
		return "overwrote"
	case Skipped:
		return "skipped"
	case Merged:
		return "merged"
	}
	return "?"
}

// FileResult is the outcome for one installed file.
type FileResult struct {
	Target string
	Src    string // path relative to build/<target>/
	Dest   string // absolute destination path
	Action Action
	Err    error
}

// Plan is an install request.
type Plan struct {
	BuildRoot      string
	Targets        []string
	Dest           string // override destination; empty = per-target default (InstallRoot + strip)
	Force          bool
	NonInteractive bool
	In             io.Reader
	Out            io.Writer
}

type runner struct {
	plan   Plan
	yesAll bool
}

// Run installs each target's build output. On existing files it prompts unless
// Force or NonInteractive; managed-region files are merged instead of clobbered.
// Returns one FileResult per file and aborts early if the user answers "q".
func Run(plan Plan) ([]FileResult, error) {
	r := runner{plan: plan, yesAll: plan.Force}
	var results []FileResult
	for _, target := range plan.Targets {
		spec, ok := compile.SpecFor(target)
		if !ok {
			continue
		}
		dest, strip := plan.Dest, ""
		if dest == "" {
			dest = expandHome(spec.InstallRoot)
			strip = spec.InstallStrip
		}
		srcRoot := filepath.Join(plan.BuildRoot, target)
		if _, err := os.Stat(srcRoot); err != nil {
			continue
		}
		abort, err := r.walkTarget(&results, target, srcRoot, dest, strip)
		if err != nil {
			return results, err
		}
		if abort {
			break
		}
	}
	return results, nil
}

func (r *runner) walkTarget(results *[]FileResult, target, srcRoot, dest, strip string) (bool, error) {
	abort := false
	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		installRel := rel
		if strip != "" && strings.HasPrefix(rel, strip) {
			installRel = rel[len(strip):]
		}
		destPath := filepath.Join(dest, installRel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return r.handle(results, target, rel, destPath, data, &abort)
	})
	return abort, err
}

func (r *runner) handle(results *[]FileResult, target, rel, destPath string, data []byte, abort *bool) error {
	if bytes.Contains(data, []byte("<!-- BEGIN ab:")) {
		existing, _ := os.ReadFile(destPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		merged := compile.MergeManagedRegions(existing, data)
		if err := os.WriteFile(destPath, merged, 0o644); err != nil {
			return err
		}
		act := Merged
		if existing == nil {
			act = Created
		}
		*results = append(*results, FileResult{target, rel, destPath, act, nil})
		return nil
	}

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0o644); err != nil {
			return err
		}
		*results = append(*results, FileResult{target, rel, destPath, Created, nil})
		return nil
	}

	switch {
	case r.yesAll:
		return r.overwrite(results, target, rel, destPath, data)
	case r.plan.NonInteractive:
		*results = append(*results, FileResult{target, rel, destPath, Skipped, nil})
		return nil
	}

	switch r.prompt(destPath) {
	case "y":
		return r.overwrite(results, target, rel, destPath, data)
	case "a":
		r.yesAll = true
		return r.overwrite(results, target, rel, destPath, data)
	case "q":
		*abort = true
		return errAbort
	default:
		*results = append(*results, FileResult{target, rel, destPath, Skipped, nil})
	}
	return nil
}

func (r *runner) overwrite(results *[]FileResult, target, rel, destPath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return err
	}
	*results = append(*results, FileResult{target, rel, destPath, Overwritten, nil})
	return nil
}

func (r *runner) prompt(destPath string) string {
	fmt.Fprintf(r.plan.Out, "overwrite %s? [y/N/a/q] ", destPath)
	scanner := bufio.NewScanner(r.plan.In)
	if !scanner.Scan() {
		return "n"
	}
	return strings.ToLower(strings.TrimSpace(scanner.Text()))
}

func expandHome(p string) string {
	if p == "" || !strings.HasPrefix(p, "~") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
