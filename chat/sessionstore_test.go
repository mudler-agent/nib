package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestSessionStoreRoundTrip(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	rec := SessionRecord{
		ID:      "abc123",
		Title:   "what changed in the last commit?",
		Cwd:     "/home/u/proj",
		Model:   "gpt-4",
		Created: time.Now().Truncate(time.Second),
		Updated: time.Now().Truncate(time.Second),
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != rec.Title || len(got.Messages) != 2 || got.Messages[1].Content != "hi" {
		t.Errorf("round trip lost data: %+v", got)
	}
}

// TestSessionStoreListFiltersByCwd: /resume is cwd-scoped by default so the
// picker stays short and relevant; --all widens it.
func TestSessionStoreListFiltersByCwd(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	mustSave(t, store, SessionRecord{ID: "a", Cwd: "/proj/one", Updated: time.Now()})
	mustSave(t, store, SessionRecord{ID: "b", Cwd: "/proj/two", Updated: time.Now()})

	here, err := store.List("/proj/one")
	if err != nil {
		t.Fatal(err)
	}
	if len(here) != 1 || here[0].ID != "a" {
		t.Errorf("cwd-filtered list = %v, want just session a", here)
	}

	all, err := store.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("unfiltered list has %d sessions, want 2", len(all))
	}
}

// TestSessionStoreListIsNewestFirst: the picker's first row should be the
// session the user most likely wants.
func TestSessionStoreListIsNewestFirst(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	old := time.Now().Add(-time.Hour)
	mustSave(t, store, SessionRecord{ID: "old", Cwd: "/p", Updated: old})
	mustSave(t, store, SessionRecord{ID: "new", Cwd: "/p", Updated: time.Now()})

	got, err := store.List("/p")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "new" {
		t.Errorf("list not newest-first: %v", got)
	}
}

// TestSessionStoreRoundTripPreservesToolCalls proves the message shape most
// likely to lose data silently — an assistant message carrying ToolCalls
// (with a ToolCallID and a function name/arguments) and the matching
// tool-role reply — survives Save/Load intact. /resume's whole promise is
// LOSSLESS restoration; encoding/json round-trips every exported field on
// openai.ChatCompletionMessage (including via its custom Marshal/Unmarshal),
// but that is a structural argument, not evidence, until a test exercises it.
func TestSessionStoreRoundTripPreservesToolCalls(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	rec := SessionRecord{
		ID: "toolcalls1",
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "what's in main.go?"},
			{
				Role: "assistant",
				ToolCalls: []openai.ToolCall{{
					ID:   "call_1",
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"main.go"}`,
					},
				}},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "package main\n"},
		},
	}
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("toolcalls1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("round trip lost messages: got %d, want 3: %+v", len(got.Messages), got.Messages)
	}

	assistant := got.Messages[1]
	if len(assistant.ToolCalls) != 1 {
		t.Fatalf("ToolCalls lost: %+v", assistant)
	}
	call := assistant.ToolCalls[0]
	if call.ID != "call_1" || call.Function.Name != "read_file" || call.Function.Arguments != `{"path":"main.go"}` {
		t.Errorf("ToolCall round trip lost data: %+v", call)
	}

	toolReply := got.Messages[2]
	if toolReply.Role != "tool" || toolReply.ToolCallID != "call_1" || toolReply.Content != "package main\n" {
		t.Errorf("tool-role reply round trip lost data: %+v", toolReply)
	}
}

// TestSessionStoreSaveCreatesPrivateDirectory proves the sessions directory
// is created 0700, not the more permissive 0755 an earlier version used: a
// session FILE being 0600 (os.CreateTemp's default, preserved through
// Save's rename) protects the transcript content, but a world/group-
// readable directory would still let any local user enumerate session ids
// and their timestamps across every project this user has ever run nib in —
// a real privacy leak the directory mode alone controls. Matches the
// existing precedent for sensitive local state: loop/persist.go and
// setup/write.go both use 0700 for the directory holding what they write.
func TestSessionStoreSaveCreatesPrivateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	store := NewSessionStore(dir)
	mustSave(t, store, SessionRecord{ID: "a", Cwd: "/p", Updated: time.Now()})

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("sessions directory mode = %o, want 0700", got)
	}
}

// TestSessionStoreSaveTightensExistingPermissiveDirectory covers the case
// os.MkdirAll alone cannot: MkdirAll does NOT chmod a directory that already
// exists. A sessions directory created 0755 by an earlier build of this
// branch (before the 0700 fix) would stay world-readable forever — these
// files hold full conversation transcripts — unless Save actively tightens
// it back down on every call.
func TestSessionStoreSaveTightensExistingPermissiveDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	store := NewSessionStore(dir)
	mustSave(t, store, SessionRecord{ID: "a", Cwd: "/p", Updated: time.Now()})

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("sessions directory mode after Save = %o, want 0700 (Save must tighten a pre-existing permissive directory)", got)
	}
}

func TestSessionStoreLoadMissingIsAnError(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	if _, err := store.Load("nope"); err == nil {
		t.Error("loading an unknown session should error")
	}
}

// TestSessionStoreListSkipsCorruptFile proves one malformed session file does
// not make List (and so /resume) fail outright — it is skipped, and every
// other, well-formed session is still returned.
func TestSessionStoreListSkipsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	mustSave(t, store, SessionRecord{ID: "good", Cwd: "/p", Updated: time.Now()})

	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := store.List("")
	if err != nil {
		t.Fatalf("List returned an error for a corrupt file, want it skipped: %v", err)
	}
	if len(got) != 1 || got[0].ID != "good" {
		t.Errorf("List = %v, want just the one good session", got)
	}
}

// TestSessionStoreSaveLeavesNoTempFile proves Save's atomic write (temp file
// + rename) does not litter the directory with the intermediate file it used
// to get there.
func TestSessionStoreSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	mustSave(t, store, SessionRecord{ID: "abc", Cwd: "/p", Updated: time.Now()})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "abc.json" {
		t.Errorf("directory after Save = %v, want exactly [abc.json]", entries)
	}
}

func mustSave(t *testing.T, s *SessionStore, rec SessionRecord) {
	t.Helper()
	if err := s.Save(rec); err != nil {
		t.Fatal(err)
	}
}

// TestSessionStoreDefaultMaxSessionsIs200 pins the decided default (Task 20
// brief): a store with MaxSessions unset caps at 200, not unlimited.
func TestSessionStoreDefaultMaxSessionsIs200(t *testing.T) {
	if DefaultMaxSessions != 200 {
		t.Fatalf("DefaultMaxSessions = %d, want 200", DefaultMaxSessions)
	}
	store := NewSessionStore(t.TempDir())
	if got := store.maxSessions(); got != DefaultMaxSessions {
		t.Errorf("maxSessions() with MaxSessions unset = %d, want %d", got, DefaultMaxSessions)
	}
}

// TestSessionStoreMaxSessionsIsConfigurable proves the cap can be overridden
// per store (the config knob wires this field — see types.Config.SessionRetention).
func TestSessionStoreMaxSessionsIsConfigurable(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	store.MaxSessions = 5
	if got := store.maxSessions(); got != 5 {
		t.Errorf("maxSessions() with MaxSessions=5 = %d, want 5", got)
	}
}

// TestSessionStoreSavePrunesBeyondCap proves Save prunes down to the
// configured cap, keeping the most recently written sessions. Ordering here
// relies on each sequential Save producing a strictly newer file mtime than
// the last (nanosecond-resolution filesystem timestamps), which is what
// prune actually keys off — see prune's doc comment for why content-level
// Updated is not used.
func TestSessionStoreSavePrunesBeyondCap(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	// Large during setup: nothing should be pruned yet while mtimes below are
	// being staggered explicitly. Real wall-clock mtimes are not trustworthy
	// enough here to order five back-to-back saves (some filesystems/CI
	// sandboxes truncate mtime resolution), so pin them with os.Chtimes
	// instead of relying on save order.
	store.MaxSessions = 100

	ids := []string{"a", "b", "c", "d", "e"}
	base := time.Now().Add(-time.Hour)
	for i, id := range ids {
		mustSave(t, store, SessionRecord{ID: id, Cwd: "/p"})
		mt := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(filepath.Join(dir, id+".json"), mt, mt); err != nil {
			t.Fatal(err)
		}
	}

	// Now impose the real cap and trigger one more prune pass by saving a
	// sixth session, whose real (current) mtime is newer than every staggered
	// one above.
	store.MaxSessions = 3
	mustSave(t, store, SessionRecord{ID: "f", Cwd: "/p"})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("directory has %d files after pruning to cap 3, want 3: %v", len(entries), entries)
	}
	for _, want := range []string{"d.json", "e.json", "f.json"} {
		found := false
		for _, e := range entries {
			if e.Name() == want {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %s to survive pruning (newest 3), directory = %v", want, entries)
		}
	}
	for _, gone := range []string{"a.json", "b.json", "c.json"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be pruned away, stat err = %v", gone, err)
		}
	}
}

// TestSessionStoreSaveAtCapDoesNotPrune covers the "exactly at cap" boundary
// that TestSessionStoreSavePrunesBeyondCap does not exercise: the exempted
// session being written COUNTS toward the cap, so with MaxSessions=3, saving
// a 3rd session must leave all 3 on disk rather than pruning down to 2.
func TestSessionStoreSaveAtCapDoesNotPrune(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	// Large during setup, same reasoning as TestSessionStoreSavePrunesBeyondCap:
	// nothing should be pruned while the first two mtimes are staggered by hand.
	store.MaxSessions = 100

	base := time.Now().Add(-time.Hour)
	for i, id := range []string{"a", "b"} {
		mustSave(t, store, SessionRecord{ID: id, Cwd: "/p"})
		mt := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(filepath.Join(dir, id+".json"), mt, mt); err != nil {
			t.Fatal(err)
		}
	}

	// Now impose the real cap and save the 3rd session: disk settles at
	// exactly MaxSessions (3), so nothing should be pruned.
	store.MaxSessions = 3
	mustSave(t, store, SessionRecord{ID: "c", Cwd: "/p"})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("directory has %d files at exactly the cap, want 3 (no pruning): %v", len(entries), entries)
	}
	for _, want := range []string{"a.json", "b.json", "c.json"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s to survive (at cap, nothing pruned): %v", want, err)
		}
	}
}

// TestSessionStoreSaveOneUnderCapDoesNotPrune covers the cap-1 boundary:
// saving a 2nd session under a cap of 3 must not prune anything either.
func TestSessionStoreSaveOneUnderCapDoesNotPrune(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	store.MaxSessions = 100
	mustSave(t, store, SessionRecord{ID: "a", Cwd: "/p"})

	store.MaxSessions = 3
	mustSave(t, store, SessionRecord{ID: "b", Cwd: "/p"})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("directory has %d files one under the cap, want 2 (no pruning): %v", len(entries), entries)
	}
	for _, want := range []string{"a.json", "b.json"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s to survive (under cap, nothing pruned): %v", want, err)
		}
	}
}

// TestSessionStorePruneNeverDeletesTheExemptSession is the sharpest form of
// the Task 20 brief's central danger: "pruning must never delete the session
// currently being written." A naive prune (sort everyone including the
// current session by recency, drop the tail) would delete the just-written
// file if its timestamp did not happen to sort as newest — e.g. clock skew,
// or (as forced here) a filesystem that reports a stale mtime for it. prune's
// keepID parameter must exclude it from candidacy outright, not rely on it
// naturally sorting to the top.
func TestSessionStorePruneNeverDeletesTheExemptSession(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	// Large during setup so these three saves' own automatic pruning does not
	// remove anything before the real test (a manual, tightly-capped prune
	// call below) runs.
	store.MaxSessions = 100

	mustSave(t, store, SessionRecord{ID: "current", Cwd: "/p"})
	mustSave(t, store, SessionRecord{ID: "other-a", Cwd: "/p"})
	mustSave(t, store, SessionRecord{ID: "other-b", Cwd: "/p"})

	// Make "current" look like the OLDEST file on disk — everything else is
	// now unambiguously newer than it by mtime.
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "current.json"), old, old); err != nil {
		t.Fatal(err)
	}

	// Exercise prune directly with "current" as the exempt id and a tight
	// cap, exactly as Save(rec) would for rec.ID == "current" — this is the
	// call a save-in-progress makes about itself.
	store.MaxSessions = 1
	store.prune("current")

	if _, err := os.Stat(filepath.Join(dir, "current.json")); err != nil {
		t.Fatalf("prune deleted the exempt current session despite it looking oldest: %v", err)
	}
}

// TestSessionStorePruneSkipsCorruptFile proves a malformed session file does
// not block a prune pass (mirroring TestSessionStoreListSkipsCorruptFile for
// List): Save must still succeed AND still actually prune down to cap despite
// an unparseable file sitting in the directory. prune never parses JSON (it
// sorts by file mtime, not content — see its doc comment) and treats a
// ".json" file as a candidate regardless of whether it parses, so with
// MaxSessions=1 the pre-existing corrupt.json is exactly the one file prune
// has budget to keep beyond keepID — asserting Save's error alone (the
// original form of this test) passes whether prune deleted everything,
// nothing, or just the corrupt file, since Save swallows every prune outcome
// by design. Asserting which files remain on disk closes that gap.
func TestSessionStorePruneSkipsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	store.MaxSessions = 1

	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.Save(SessionRecord{ID: "current", Cwd: "/p"}); err != nil {
		t.Fatalf("Save returned an error because of an unrelated corrupt file: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "current.json")); err != nil {
		t.Errorf("current.json (keepID, just saved) should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "corrupt.json")); !os.IsNotExist(err) {
		t.Errorf("corrupt.json should have been pruned (MaxSessions=1 leaves no budget beyond keepID), stat err = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var jsonFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}
	if len(jsonFiles) != 1 || jsonFiles[0] != "current.json" {
		t.Errorf("want exactly [current.json] left, got %v", jsonFiles)
	}
}

// TestSessionStoreDeleteRemovesExactlyOneFile proves Delete removes only the
// named session's file, leaving the rest of the store untouched.
func TestSessionStoreDeleteRemovesExactlyOneFile(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	mustSave(t, store, SessionRecord{ID: "a", Cwd: "/p"})
	mustSave(t, store, SessionRecord{ID: "b", Cwd: "/p"})
	mustSave(t, store, SessionRecord{ID: "c", Cwd: "/p"})

	if err := store.Delete("b"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.json")); !os.IsNotExist(err) {
		t.Errorf("Delete did not remove b.json: stat err = %v", err)
	}
	for _, id := range []string{"a", "c"} {
		if _, err := os.Stat(filepath.Join(dir, id+".json")); err != nil {
			t.Errorf("Delete removed an unrelated file %s: %v", id, err)
		}
	}
}

// TestSessionStoreDeleteMissingIsNotAnError matches Delete's non-fatal
// contract to the rest of the store: deleting an already-gone session (e.g. a
// picker racing a concurrent prune) is not an error.
func TestSessionStoreDeleteMissingIsNotAnError(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	if err := store.Delete("nope"); err != nil {
		t.Errorf("deleting an already-gone session should not error, got %v", err)
	}
}
