package slash

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mudler/nib/types"
)

func TestExpand(t *testing.T) {
	out, err := Expand(types.CommandConfig{Prompt: "Review: {{.Args}}"}, "the diff")
	if err != nil || out != "Review: the diff" {
		t.Fatalf("expand: %q err %v", out, err)
	}
}

func TestResolve(t *testing.T) {
	cmds := []types.CommandConfig{
		{Name: "review", Prompt: "Review: {{.Args}}"},
		{Name: "scan", Prompt: "Scan it", Agent: "explore"},
	}
	skills := []types.Skill{{Name: "git-commit", Instructions: "body"}}
	agents := []types.AgentTypeConfig{{Name: "explore"}}

	if a := Resolve("hello world", cmds, skills, agents); a.Kind != KindSend || a.Text != "hello world" {
		t.Fatalf("plain: %+v", a)
	}
	if a := Resolve("/review the diff", cmds, skills, agents); a.Kind != KindSend || a.Text != "Review: the diff" {
		t.Fatalf("command: %+v", a)
	}
	a := Resolve("/scan", cmds, skills, agents)
	if a.Kind != KindSend || !strings.Contains(a.Text, "explore") || !strings.Contains(a.Text, "Scan it") {
		t.Fatalf("agent-bound command: %+v", a)
	}
	if a := Resolve("/skill git-commit", cmds, skills, agents); a.Kind != KindLoadSkill || a.Skill != "git-commit" {
		t.Fatalf("skill: %+v", a)
	}
	if a := Resolve("/skill nope", cmds, skills, agents); a.Kind != KindError {
		t.Fatalf("unknown skill should error: %+v", a)
	}
	if a := Resolve("/skill", cmds, skills, agents); a.Kind != KindError {
		t.Fatalf("skill with no name should error: %+v", a)
	}
	if a := Resolve("/agent explore find bugs", cmds, skills, agents); a.Kind != KindSend || !strings.Contains(a.Text, "explore") || !strings.Contains(a.Text, "find bugs") {
		t.Fatalf("agent: %+v", a)
	}
	if a := Resolve("/agent ghost x", cmds, skills, agents); a.Kind != KindError {
		t.Fatalf("unknown agent should error: %+v", a)
	}
	if a := Resolve("/bogus", cmds, skills, agents); a.Kind != KindError {
		t.Fatalf("unknown command should error: %+v", a)
	}
}

func TestResolveCompact(t *testing.T) {
	got := Resolve("/compact", nil, nil, nil)
	if got.Kind != KindCompact {
		t.Fatalf("/compact resolved to kind %v, want KindCompact", got.Kind)
	}
}

func TestResolveLoop(t *testing.T) {
	var none []types.CommandConfig
	var noSkills []types.Skill
	var noAgents []types.AgentTypeConfig

	// Fixed interval: "/loop 5m /foo".
	a := Resolve("/loop 5m /foo", none, noSkills, noAgents)
	if a.Kind != KindLoopStart || a.Interval != 5*time.Minute || a.Payload != "/foo" {
		t.Fatalf("fixed: %+v", a)
	}

	// Self-paced: "/loop /foo" (no parseable interval → interval 0).
	a = Resolve("/loop /foo", none, noSkills, noAgents)
	if a.Kind != KindLoopStart || a.Interval != 0 || a.Payload != "/foo" {
		t.Fatalf("self-paced: %+v", a)
	}

	// 1s is at the floor now → NOT clamped.
	a = Resolve("/loop 1s ping", none, noSkills, noAgents)
	if a.Kind != KindLoopStart || a.Interval != 1*time.Second {
		t.Fatalf("1s floor: %+v", a)
	}

	// Sub-second interval is clamped up to the 1s floor.
	a = Resolve("/loop 500ms ping", none, noSkills, noAgents)
	if a.Kind != KindLoopStart || a.Interval != 1*time.Second {
		t.Fatalf("clamp: %+v", a)
	}

	// Control verbs.
	if a := Resolve("/loop stop", none, noSkills, noAgents); a.Kind != KindLoopStop || a.LoopID != "" {
		t.Fatalf("stop-all: %+v", a)
	}
	if a := Resolve("/loop stop loop-2", none, noSkills, noAgents); a.Kind != KindLoopStop || a.LoopID != "loop-2" {
		t.Fatalf("stop-id: %+v", a)
	}
	if a := Resolve("/loop list", none, noSkills, noAgents); a.Kind != KindLoopList {
		t.Fatalf("list: %+v", a)
	}

	// Empty payload → error.
	if a := Resolve("/loop", none, noSkills, noAgents); a.Kind != KindError {
		t.Fatalf("empty: %+v", a)
	}
	if a := Resolve("/loop 5m", none, noSkills, noAgents); a.Kind != KindError {
		t.Fatalf("interval-only: %+v", a)
	}
}

func TestResolveModel(t *testing.T) {
	var (
		cmds   []types.CommandConfig
		skills []types.Skill
		agents []types.AgentTypeConfig
	)

	cases := []struct {
		input string
		kind  Kind
		model string
	}{
		{input: "/models", kind: KindModelList},
		{input: "/model", kind: KindModelPick},
		{input: "/model qwen3.5-4b", kind: KindModelSet, model: "qwen3.5-4b"},
	}
	for _, tc := range cases {
		a := Resolve(tc.input, cmds, skills, agents)
		if a.Kind != tc.kind || a.Model != tc.model {
			t.Errorf("Resolve(%q) = %+v, want kind %v and model %q", tc.input, a, tc.kind, tc.model)
		}
	}

	if a := Resolve("/model   spaced-name  ", cmds, skills, agents); a.Kind != KindModelSet || a.Model != "spaced-name" {
		t.Fatalf("/model should trim: %+v", a)
	}
}

func TestResolveGoal(t *testing.T) {
	cases := []struct {
		in   string
		want Action
	}{
		{"/goal make all tests pass", Action{Kind: KindGoalSet, Text: "make all tests pass"}},
		{"/goal", Action{Kind: KindGoalShow}},
		{"/goal clear", Action{Kind: KindGoalClear}},
		{"/goal   ", Action{Kind: KindGoalShow}},
	}
	for _, c := range cases {
		got := Resolve(c.in, nil, nil, nil)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Resolve(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestResolveYolo(t *testing.T) {
	on, off := true, false
	cases := []struct {
		in   string
		want *bool
	}{
		{"/yolo", nil},
		{"/yolo on", &on},
		{"/yolo off", &off},
	}
	for _, c := range cases {
		got := Resolve(c.in, nil, nil, nil)
		if got.Kind != KindYolo {
			t.Fatalf("%q: Kind = %v, want KindYolo", c.in, got.Kind)
		}
		switch {
		case c.want == nil && got.YoloOn != nil:
			t.Errorf("%q: YoloOn = %v, want nil (toggle)", c.in, *got.YoloOn)
		case c.want != nil && (got.YoloOn == nil || *got.YoloOn != *c.want):
			t.Errorf("%q: YoloOn = %v, want %v", c.in, got.YoloOn, *c.want)
		}
	}
}

func TestResolveYoloInvalid(t *testing.T) {
	got := Resolve("/yolo bogus", nil, nil, nil)
	if got.Kind != KindError {
		t.Fatalf("Kind = %v, want KindError", got.Kind)
	}
}

func TestResolveResume(t *testing.T) {
	cases := []struct {
		in      string
		wantAll bool
		wantID  string
	}{
		{"/resume", false, ""},
		{"/resume --all", true, ""},
		{"/resume abc123", false, "abc123"},
	}
	for _, c := range cases {
		got := Resolve(c.in, nil, nil, nil)
		if got.Kind != KindResume {
			t.Fatalf("%q: Kind = %v, want KindResume", c.in, got.Kind)
		}
		if got.ResumeAll != c.wantAll || got.ResumeID != c.wantID {
			t.Errorf("%q: all=%v id=%q, want all=%v id=%q", c.in, got.ResumeAll, got.ResumeID, c.wantAll, c.wantID)
		}
	}
}
