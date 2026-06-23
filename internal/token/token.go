// Package token resolves the canonical token grammar against a per-target
// Runtime, producing target-specific body text.
package token

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"agent-builder/internal/canon"
)

// Runtime maps the portable runtime tokens ({{input}}, {{arg:NAME}},
// {{rules_file}}, {{selection}}) to a target's concrete syntax.
type Runtime interface {
	Input() string
	Arg(name string) string
	Selection() string
	RulesFile() string
}

// Resolver is invoked when a tool token has no known @server, giving the
// caller a chance to supply one (e.g. by prompting). Resolve registers the
// server on the registry so a re-resolution succeeds.
type Resolver interface {
	Resolve(toolID, target string)
}

type tokenReplacer struct {
	rt       Runtime
	tools    *canon.ToolRegistry
	target   string
	resolver Resolver
}

var bodyTokenRe = regexp.MustCompile(`\{\{([^{}]+)\}\}`)

// RenderBody scans body for tokens and returns the text with every token
// resolved for the given target. Unknown tokens are left untouched; tools
// without a resolvable server produce an error (unless resolver handles it).
func RenderBody(body string, rt Runtime, tools *canon.ToolRegistry, target string, resolver Resolver) (string, error) {
	r := tokenReplacer{rt: rt, tools: tools, target: target, resolver: resolver}
	var b strings.Builder
	last := 0
	for _, m := range bodyTokenRe.FindAllStringSubmatchIndex(body, -1) {
		b.WriteString(body[last:m[0]])
		inner := strings.TrimSpace(body[m[2]:m[3]])
		out, err := r.renderToken(inner)
		if err != nil {
			return "", fmt.Errorf("token {{%s}}: %w", inner, err)
		}
		b.WriteString(out)
		last = m[1]
	}
	b.WriteString(body[last:])
	return b.String(), nil
}

func (r *tokenReplacer) renderToken(inner string) (string, error) {
	switch {
	case inner == "input":
		return r.rt.Input(), nil
	case inner == "rules_file":
		return r.rt.RulesFile(), nil
	case inner == "selection":
		return r.rt.Selection(), nil
	case strings.HasPrefix(inner, "arg:"):
		return r.rt.Arg(strings.TrimSpace(inner[4:])), nil
	case strings.HasPrefix(inner, "tool "):
		return r.renderTool(strings.TrimSpace(inner[5:]))
	case strings.HasPrefix(inner, "skill "):
		return fmt.Sprintf("@%s", strings.TrimSpace(inner[6:])), nil
	default:
		return "{{" + inner + "}}", nil
	}
}

func (r *tokenReplacer) renderTool(spec string) (string, error) {
	fields, err := splitFields(spec)
	if err != nil {
		return "", err
	}
	if len(fields) == 0 {
		return "", fmt.Errorf("empty tool reference")
	}
	toolID := fields[0]
	if at := strings.IndexByte(toolID, '@'); at >= 0 {
		server := toolID[at+1:]
		toolID = toolID[:at]
		if server != "" {
			r.tools.SetServer(toolID, server)
		}
	}

	var args []argPair
	for _, kv := range fields[1:] {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			return "", fmt.Errorf("tool %s: argument %q must be key=value", toolID, kv)
		}
		args = append(args, argPair{K: kv[:eq], V: kv[eq+1:]})
	}

	tmpl, reach, derived := r.tools.Resolve(toolID, r.target)
	if reach == canon.Undefined && r.resolver != nil {
		r.resolver.Resolve(toolID, r.target)
		tmpl, reach, derived = r.tools.Resolve(toolID, r.target)
	}
	switch reach {
	case canon.Unreachable:
		return fmt.Sprintf("[tool %s not available on %s]", toolID, r.target), nil
	case canon.Undefined:
		return "", fmt.Errorf("tool %s has no @server declared and none known — add @<server> to the token (e.g. {{tool %s@fortix ...}}) or run interactively", toolID, toolID)
	}
	_ = derived
	return r.renderDerived(tmpl, toolID, args), nil
}

type argPair struct {
	K, V string
}

func (r *tokenReplacer) renderDerived(tmpl, toolID string, args []argPair) string {
	out := tmpl
	out = strings.ReplaceAll(out, "{server}", r.tools.Server(toolID))
	out = strings.ReplaceAll(out, "{tool}", toolID)
	out = strings.ReplaceAll(out, "{params}", r.renderParams(args))
	return out
}

func (r *tokenReplacer) renderParams(args []argPair) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.K + "=" + r.renderValue(a.V)
	}
	return strings.Join(parts, " ")
}

func (r *tokenReplacer) renderValue(val string) string {
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		return val[1 : len(val)-1]
	}
	switch {
	case val == "input":
		return r.rt.Input()
	case val == "selection":
		return r.rt.Selection()
	case strings.HasPrefix(val, "arg:"):
		return r.rt.Arg(strings.TrimSpace(val[4:]))
	default:
		return val
	}
}

func splitFields(s string) ([]string, error) {
	var fields []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
			cur.WriteByte(c)
		case (c == ' ' || c == '\t' || c == '\n') && !inQuote:
			if cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields, nil
}

// MarkdownRuntime renders runtime tokens for the slash-command targets
// (claude, opencode, codex, kiro): $ARGUMENTS and positional $N.
type MarkdownRuntime struct {
	Args         []canon.Argument
	ArgIndexBase int
	RulesFileVal string
}

func (r MarkdownRuntime) Input() string     { return "$ARGUMENTS" }
func (r MarkdownRuntime) Selection() string { return "$ARGUMENTS" }
func (r MarkdownRuntime) RulesFile() string { return r.RulesFileVal }

func (r MarkdownRuntime) Arg(name string) string {
	if len(r.Args) == 1 {
		return "$ARGUMENTS"
	}
	for i, a := range r.Args {
		if a.Name == name {
			return "$" + strconv.Itoa(i+r.ArgIndexBase)
		}
	}
	return "$ARGUMENTS"
}

// CopilotRuntime renders runtime tokens for Copilot prompt files using
// ${input:NAME} variables.
type CopilotRuntime struct {
	Args []canon.Argument
}

func (r CopilotRuntime) Input() string     { return "${input:input}" }
func (r CopilotRuntime) Selection() string { return "${selection}" }
func (r CopilotRuntime) RulesFile() string { return ".github/copilot-instructions.md" }

func (r CopilotRuntime) Arg(name string) string {
	return "${input:" + name + "}"
}

// ValidateBody checks that every {{tool}} token in body resolves on the
// target (i.e. has a known @server). Non-tool tokens are not checked.
func ValidateBody(body string, tools *canon.ToolRegistry, target string) error {
	for _, m := range bodyTokenRe.FindAllStringSubmatchIndex(body, -1) {
		inner := strings.TrimSpace(body[m[2]:m[3]])
		if !strings.HasPrefix(inner, "tool ") {
			continue
		}
		fields, err := splitFields(strings.TrimSpace(inner[5:]))
		if err != nil {
			return fmt.Errorf("token {{%s}}: %w", inner, err)
		}
		if len(fields) == 0 {
			continue
		}
		toolID := fields[0]
		if at := strings.IndexByte(toolID, '@'); at >= 0 {
			server := toolID[at+1:]
			toolID = toolID[:at]
			if server != "" {
				tools.SetServer(toolID, server)
			}
		}
		if _, reach, _ := tools.Resolve(toolID, target); reach == canon.Undefined {
			return fmt.Errorf("token {{%s}}: tool %q has no @server declared — add @<server> to the token", inner, toolID)
		}
	}
	return nil
}
