package app

import (
	"path/filepath"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/plugin"
	"github.com/mudler/nib/types"
)

// seededCreated is the Created stamp every seeded record carries.
var seededCreated = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

// seedSession writes one recorded session into the store applySessionIdentity
// reads, rooted at the same BaseDir the caller passes through types.Config.
func seedSession(t *testing.T, baseDir, id string) {
	t.Helper()
	store := chat.NewSessionStore(filepath.Join(plugin.BaseDirIn(baseDir), "sessions"))
	err := store.Save(chat.SessionRecord{
		ID:       id,
		Title:    "seeded",
		Created:  seededCreated,
		Updated:  time.Now(),
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// An external supervisor (voro, a wrapper script) needs to name the session
// BEFORE it exists, so that the id it spawned with is the id it can resume
// with later. Without --session-id the TUI mints an id nothing else can see.
func TestSessionIDPinsTheRecordID(t *testing.T) {
	cfg := types.Config{BaseDir: t.TempDir()}
	if err := applySessionIdentity(&cfg, "pinned-1", false, false, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ResumeSessionID != "pinned-1" {
		t.Fatalf("ResumeSessionID = %q, want pinned-1", cfg.ResumeSessionID)
	}
	if len(cfg.InitialHistory) != 0 {
		t.Fatalf("pinning an id must not seed history: %v", cfg.InitialHistory)
	}
}

// The supervisor's resume: --session-id names WHICH session, so the id never
// has to travel as a bare argument (a bare arg would swallow every flag after
// it, since Go's flag package stops parsing at the first non-flag).
func TestResumeWithSessionIDLoadsThatSession(t *testing.T) {
	base := t.TempDir()
	seedSession(t, base, "pinned-2")

	cfg := types.Config{BaseDir: base}
	if err := applySessionIdentity(&cfg, "pinned-2", true, false, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ResumeSessionID != "pinned-2" {
		t.Fatalf("ResumeSessionID = %q, want pinned-2", cfg.ResumeSessionID)
	}
	if len(cfg.InitialHistory) != 1 {
		t.Fatalf("history not seeded: %v", cfg.InitialHistory)
	}
	if cfg.ResumeSessionTitle != "seeded" {
		t.Fatalf("title = %q, want seeded", cfg.ResumeSessionTitle)
	}
	if !cfg.ResumeSessionCreated.Equal(seededCreated) {
		t.Fatalf("created = %v, want %v", cfg.ResumeSessionCreated, seededCreated)
	}
}

// A bare argument still wins, so `nib --resume abc` keeps working for humans.
func TestBareArgumentOutranksSessionIDOnResume(t *testing.T) {
	base := t.TempDir()
	seedSession(t, base, "bare-3")

	cfg := types.Config{BaseDir: base}
	if err := applySessionIdentity(&cfg, "pinned-3", true, false, []string{"bare-3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ResumeSessionID != "bare-3" {
		t.Fatalf("ResumeSessionID = %q, want bare-3", cfg.ResumeSessionID)
	}
}

// The first restart of a session that never reached a turn boundary finds
// nothing to load. That is not fatal, and the id must still be pinned —
// otherwise the fresh session forks a new id and the NEXT restart is lost too.
func TestFailedResumeStillPinsTheSessionID(t *testing.T) {
	cfg := types.Config{BaseDir: t.TempDir()}
	err := applySessionIdentity(&cfg, "pinned-4", true, false, nil)
	if err == nil {
		t.Fatal("want an error for an unrecorded session id")
	}
	if cfg.ResumeSessionID != "pinned-4" {
		t.Fatalf("ResumeSessionID = %q, want pinned-4", cfg.ResumeSessionID)
	}
}

// Bare `nib --resume` with nothing recorded is unchanged: report, mint fresh.
func TestBareResumeWithNothingRecordedPinsNothing(t *testing.T) {
	cfg := types.Config{BaseDir: t.TempDir()}
	if err := applySessionIdentity(&cfg, "", true, false, nil); err == nil {
		t.Fatal("want an error when no sessions are recorded")
	}
	if cfg.ResumeSessionID != "" {
		t.Fatalf("ResumeSessionID = %q, want empty", cfg.ResumeSessionID)
	}
}

// --resume brings the session's goal back with its history.
func TestResumeRestoresTheGoal(t *testing.T) {
	base := t.TempDir()
	store := chat.NewSessionStore(filepath.Join(plugin.BaseDirIn(base), "sessions"))
	err := store.Save(chat.SessionRecord{
		ID:         "goal-1",
		Updated:    time.Now(),
		Messages:   []openai.ChatCompletionMessage{{Role: "user", Content: "hi"}},
		Goal:       "ship it",
		GoalPaused: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := types.Config{BaseDir: base}
	if err := applySessionIdentity(&cfg, "goal-1", true, false, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.InitialGoal != "ship it" || !cfg.InitialGoalPaused {
		t.Fatalf("goal = %q paused = %v, want ship it, paused", cfg.InitialGoal, cfg.InitialGoalPaused)
	}
}
