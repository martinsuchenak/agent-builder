package token

import (
	"strings"
	"testing"

	"agent-builder/internal/canon"
)

func newReg() *canon.ToolRegistry { return canon.NewToolRegistry() }

func TestMarkdownRuntime(t *testing.T) {
	r := MarkdownRuntime{
		Args:         []canon.Argument{{Name: "x"}, {Name: "y"}},
		ArgIndexBase: 0,
		RulesFileVal: "CLAUDE.md",
	}
	if r.Input() != "$ARGUMENTS" || r.Selection() != "$ARGUMENTS" || r.RulesFile() != "CLAUDE.md" {
		t.Fatal("basic runtime values wrong")
	}
	if r.Arg("x") != "$0" || r.Arg("y") != "$1" {
		t.Fatalf("0-indexed args wrong: %q %q", r.Arg("x"), r.Arg("y"))
	}
	r.ArgIndexBase = 1
	if r.Arg("x") != "$1" || r.Arg("y") != "$2" {
		t.Fatalf("1-indexed args wrong: %q %q", r.Arg("x"), r.Arg("y"))
	}
	r.Args = []canon.Argument{{Name: "only"}}
	if r.Arg("only") != "$ARGUMENTS" {
		t.Fatalf("single-arg should collapse to $ARGUMENTS, got %q", r.Arg("only"))
	}
	if r.Arg("missing") != "$ARGUMENTS" {
		t.Fatalf("unknown arg should fall back to $ARGUMENTS, got %q", r.Arg("missing"))
	}
}

func TestCopilotRuntime(t *testing.T) {
	r := CopilotRuntime{}
	if r.Input() != "${input:input}" || r.Selection() != "${selection}" || r.RulesFile() != ".github/copilot-instructions.md" {
		t.Fatal("copilot runtime values wrong")
	}
	if r.Arg("foo") != "${input:foo}" {
		t.Fatalf("copilot arg wrong: %q", r.Arg("foo"))
	}
}

func TestSplitFields(t *testing.T) {
	cases := []struct {
		in   string
		want []string
		errW bool
	}{
		{`a b c`, []string{"a", "b", "c"}, false},
		{`a="x y" b`, []string{`"x y"`, "a=", "b"}[0:0], false}, // placeholder, replaced below
		{`name=fortix__x top=5`, []string{"name=fortix__x", "top=5"}, false},
		{`q="unterminated`, nil, true},
	}
	cases[1] = struct {
		in   string
		want []string
		errW bool
	}{`a="x y" b`, []string{`a="x y"`, "b"}, false}

	for _, c := range cases {
		got, err := splitFields(c.in)
		if c.errW {
			if err == nil {
				t.Errorf("splitFields(%q) expected error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitFields(%q) unexpected error: %v", c.in, err)
			continue
		}
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("splitFields(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRenderValue(t *testing.T) {
	r := tokenReplacer{rt: MarkdownRuntime{RulesFileVal: "CLAUDE.md"}}
	cases := map[string]string{
		"input":        "$ARGUMENTS",
		"selection":    "$ARGUMENTS",
		"arg:foo":      "$ARGUMENTS",
		"hello":        "hello",
		`"quoted"`:     "quoted",
		"fortix__x__y": "fortix__x__y",
	}
	for in, want := range cases {
		if got := r.renderValue(in); got != want {
			t.Errorf("renderValue(%q) = %q, want %q", in, got, want)
		}
	}
	r2 := tokenReplacer{rt: MarkdownRuntime{Args: []canon.Argument{{Name: "a"}, {Name: "b"}}, ArgIndexBase: 1}}
	if got := r2.renderValue("arg:b"); got != "$2" {
		t.Errorf("renderValue(arg:b) = %q, want $2", got)
	}
}

func TestRenderBodyTokens(t *testing.T) {
	reg := newReg()
	reg.SetServer("echo", "fortix")
	body := "input={{input}} rf={{rules_file}} sel={{selection}} a={{arg:x}} skill={{skill foo}} passthrough={{notatok}} lit={{unknown thing}}"
	rt := MarkdownRuntime{Args: []canon.Argument{{Name: "x"}, {Name: "y"}}, RulesFileVal: "CLAUDE.md"}
	out, err := RenderBody(body, rt, reg, "claude", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "input=$ARGUMENTS") {
		t.Errorf("input token: %s", out)
	}
	if !strings.Contains(out, "rf=CLAUDE.md") {
		t.Errorf("rules_file token: %s", out)
	}
	if !strings.Contains(out, "sel=$ARGUMENTS") {
		t.Errorf("selection token: %s", out)
	}
	if !strings.Contains(out, "a=$0") {
		t.Errorf("arg token: %s", out)
	}
	if !strings.Contains(out, "skill=@foo") {
		t.Errorf("skill token: %s", out)
	}
	if !strings.Contains(out, "passthrough={{notatok}}") {
		t.Errorf("unknown token should pass through: %s", out)
	}
	if !strings.Contains(out, "lit={{unknown thing}}") {
		t.Errorf("non-keyword inner should pass through: %s", out)
	}
}

func TestRenderBodyToolAtServerRegistersAndRenders(t *testing.T) {
	reg := newReg()
	body := "call {{tool echo@fortix msg=input}}"
	out, err := RenderBody(body, MarkdownRuntime{RulesFileVal: "AGENTS.md"}, reg, "opencode", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reg.Server("echo") != "fortix" {
		t.Error("@server not registered")
	}
	if !strings.Contains(out, "llmrouter_fortix__echo") || !strings.Contains(out, "msg=$ARGUMENTS") {
		t.Errorf("tool render wrong: %s", out)
	}
}

func TestRenderBodyToolRootServer(t *testing.T) {
	reg := newReg()
	body := "{{tool execute_tool@llmrouter name=fortix__echo id=input}}"
	out, err := RenderBody(body, MarkdownRuntime{RulesFileVal: "AGENTS.md"}, reg, "claude", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "mcp__llmrouter__execute_tool") || !strings.Contains(out, "name=fortix__echo") {
		t.Errorf("root-server tool render wrong: %s", out)
	}
}

func TestRenderBodyToolUnreachable(t *testing.T) {
	reg := newReg()
	reg.SetServer("echo", "fortix")
	out, err := RenderBody("{{tool echo@fortix x=1}}", MarkdownRuntime{}, reg, "codex", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[tool echo not available on codex]") {
		t.Errorf("expected unreachable marker: %s", out)
	}
}

func TestRenderBodyToolUndefinedNoResolver(t *testing.T) {
	reg := newReg()
	_, err := RenderBody("{{tool mystery x=1}}", MarkdownRuntime{}, reg, "claude", nil)
	if err == nil {
		t.Fatal("expected error for undefined tool with no resolver")
	}
}

func TestRenderBodyToolUndefinedResolver(t *testing.T) {
	reg := newReg()
	called := false
	res := stubResolver{fn: func(id, target string) { called = true; reg.SetServer(id, "fortix") }}
	out, err := RenderBody("{{tool echo msg=input}}", MarkdownRuntime{}, reg, "claude", res)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("resolver not invoked")
	}
	if !strings.Contains(out, "mcp__llmrouter__fortix__echo") {
		t.Errorf("post-resolve render wrong: %s", out)
	}
}

type stubResolver struct {
	fn func(id, target string)
}

func (s stubResolver) Resolve(id, target string) { s.fn(id, target) }

func TestRenderBodyToolBadArg(t *testing.T) {
	reg := newReg()
	_, err := RenderBody("{{tool echo@fortix notakv}}", MarkdownRuntime{}, reg, "claude", nil)
	if err == nil {
		t.Fatal("expected error for non-key=value arg")
	}
}

func TestValidateBody(t *testing.T) {
	reg := newReg()
	if err := ValidateBody("no tools here", reg, "claude"); err != nil {
		t.Errorf("plain body should validate: %v", err)
	}
	if err := ValidateBody("{{tool echo@fortix x=1}}", reg, "claude"); err != nil {
		t.Errorf("declared tool should validate: %v", err)
	}
	freshReg := newReg()
	if err := ValidateBody("{{tool echo}}", freshReg, "claude"); err == nil {
		t.Error("tool without @server should fail validation")
	}
}
