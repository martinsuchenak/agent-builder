package canon

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Reachability describes whether a tool can be invoked on a target.
type Reachability int

const (
	Reachable   Reachability = iota // the tool resolves to an invocation template
	Unreachable                     // the target marks the tool unavailable (degrades to a marker)
	Undefined                       // no server is known for the tool on this target
)

// InvocationSpec is a per-target invocation template, optionally with
// per-server overrides. Templates use the holes {server}, {tool}, {params}.
type InvocationSpec struct {
	Default  string
	ByServer map[string]string
}

// UnmarshalYAML accepts either a scalar string (the default template, or the
// sentinel "unreachable") or a mapping with an optional "default" plus
// per-server entries.
func (s *InvocationSpec) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		s.Default = value.Value
		return nil
	case yaml.MappingNode:
		s.ByServer = map[string]string{}
		for i := 0; i < len(value.Content); i += 2 {
			k := value.Content[i].Value
			v := value.Content[i+1].Value
			if k == "default" {
				s.Default = v
			} else {
				s.ByServer[k] = v
			}
		}
		return nil
	}
	return fmt.Errorf("invocation entry must be a string or mapping")
}

// MarshalYAML emits a scalar when only Default is set, otherwise a mapping.
func (s InvocationSpec) MarshalYAML() (interface{}, error) {
	if len(s.ByServer) == 0 {
		return s.Default, nil
	}
	m := map[string]string{}
	for k, v := range s.ByServer {
		m[k] = v
	}
	if s.Default != "" {
		m["default"] = s.Default
	}
	return m, nil
}

// TemplateFor returns the invocation template for the given server, falling
// back to the default.
func (s InvocationSpec) TemplateFor(server string) string {
	if s.ByServer != nil {
		if t, ok := s.ByServer[server]; ok && t != "" {
			return t
		}
	}
	return s.Default
}

// ToolRegistry holds the per-target invocation conventions plus a session map
// of tool-name -> server, populated from inline @server declarations in bodies.
// There is no central tool registry file: servers travel with the artifacts.
type ToolRegistry struct {
	Invocation map[string]InvocationSpec
	servers    map[string]string
	path       string
}

// DefaultInvocation returns the built-in per-target invocation templates.
func DefaultInvocation() map[string]InvocationSpec {
	return map[string]InvocationSpec{
		"claude": {
			Default:  "call `mcp__llmrouter__{server}__{tool}` with {params}",
			ByServer: map[string]string{"llmrouter": "call `mcp__llmrouter__{tool}` with {params}"},
		},
		"opencode": {
			Default:  "call the `llmrouter_{server}__{tool}` tool with {params}",
			ByServer: map[string]string{"llmrouter": "call the `llmrouter__{tool}` tool with {params}"},
		},
		"codex":   {Default: "unreachable"},
		"copilot": {Default: "unreachable"},
		"kiro":    {Default: "unreachable"},
	}
}

// NewToolRegistry returns a registry seeded with the built-in invocation defaults.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{Invocation: DefaultInvocation(), servers: map[string]string{}}
}

// LoadTargets loads optional per-target invocation overrides from targets.yaml.
// Built-in defaults apply for any target/key not overridden.
func LoadTargets(path string) (*ToolRegistry, error) {
	reg := NewToolRegistry()
	reg.path = path
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return nil, fmt.Errorf("read targets.yaml: %w", err)
	}
	var cfg struct {
		Invocation map[string]InvocationSpec `yaml:"invocation"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse targets.yaml: %w", err)
	}
	for k, v := range cfg.Invocation {
		reg.Invocation[k] = v
	}
	return reg, nil
}

// SetServer records the MCP server for a tool (populated from @server tokens
// or interactive prompts).
func (r *ToolRegistry) SetServer(name, server string) {
	r.servers[name] = server
}

// Server returns the recorded server for a tool, or "" if none is known.
func (r *ToolRegistry) Server(name string) string {
	return r.servers[name]
}

// Resolve looks up the invocation for a tool on a target. The returned
// template is non-empty only when Reachable; derived reports whether it came
// from an invocation template (rather than a per-tool override).
func (r *ToolRegistry) Resolve(name, target string) (template string, reach Reachability, derived bool) {
	spec, ok := r.Invocation[target]
	if !ok {
		return "", Undefined, false
	}
	server := r.servers[name]
	tmpl := spec.TemplateFor(server)
	if tmpl == "" || tmpl == "unreachable" {
		return "", Unreachable, false
	}
	if server != "" {
		return tmpl, Reachable, true
	}
	return "", Undefined, false
}
