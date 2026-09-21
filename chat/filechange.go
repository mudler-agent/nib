package chat

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"

	"github.com/mudler/nib/internal/textdiff"
)

// FileChange is what a write or edit call does to one file, captured so the UI
// can show it as a diff: before the call runs (in the approval prompt) and
// after (in the transcript).
type FileChange struct {
	Path   string
	Before string
	After  string
	// Created is true when the file did not exist before the call.
	Created bool
	// Fragment is true when Before/After are an edit's old and new strings
	// rather than the whole file, because the file could not be read or the
	// old string was not found in it. A fragment diff carries no line numbers.
	Fragment bool
}

// DiffContext is how many unchanged lines a file-change diff keeps around each
// change.
const DiffContext = 2

// Diff returns the change as a display diff.
func (c FileChange) Diff() textdiff.Diff {
	if c.Fragment {
		return textdiff.Fragment(c.Before, c.After)
	}
	return textdiff.Compute(c.Before, c.After, DiffContext)
}

// maxChangeBytes caps how large a file the UI diffs. A larger file, or a
// binary one, gets no FileChange and the UI falls back to the plain summary.
const maxChangeBytes = 1 << 20

// readForChange reads path for diffing. exists is false when the file is
// missing; ok is false when it cannot be diffed (unreadable, too large, or
// binary).
func readForChange(path string) (content string, exists, ok bool) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, true
	}
	if err != nil || info.IsDir() || info.Size() > maxChangeBytes {
		return "", true, false
	}
	b, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(b, 0) >= 0 {
		return "", true, false
	}
	return string(b), true, true
}

// PreviewFileChange predicts the change a write or edit call will make, by
// reading the target file now. Relative paths resolve against workDir, the
// same way the filesystem tools resolve them. It returns nil for any other
// tool, for arguments it cannot decode, and for files it cannot diff.
func PreviewFileChange(workDir, name, argsJSON string) *FileChange {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Old     string `json:"old"`
		New     string `json:"new"`
		All     bool   `json:"all"`
	}
	if name != "write" && name != "edit" {
		return nil
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Path == "" {
		return nil
	}
	before, exists, ok := readForChange(resolveWorkspacePath(workDir, args.Path))
	switch name {
	case "write":
		if !ok {
			return nil
		}
		return &FileChange{Path: args.Path, Before: before, After: args.Content, Created: !exists}
	default: // edit
		if !ok || !exists || args.Old == "" || !strings.Contains(before, args.Old) {
			return &FileChange{Path: args.Path, Before: args.Old, After: args.New, Fragment: true}
		}
		n := 1
		if args.All {
			n = -1
		}
		return &FileChange{Path: args.Path, Before: before, After: strings.Replace(before, args.Old, args.New, n)}
	}
}

// settle replaces a predicted After with what is actually on disk once the
// call has run. It keeps the prediction when the file cannot be read back.
func (c *FileChange) settle(workDir string) {
	if c.Fragment {
		return
	}
	if after, exists, ok := readForChange(resolveWorkspacePath(workDir, c.Path)); ok && exists {
		c.After = after
	}
}

// changeTracker holds the pre-call snapshot of each in-flight write/edit, so
// the result callback can report the change the call actually made. Entries
// are keyed by tool-call id (or name plus arguments when the id is empty) and
// removed when the result arrives.
type changeTracker struct {
	mu      sync.Mutex
	pending map[string]*FileChange
}

func changeKey(id, name, argsJSON string) string {
	if id != "" {
		return id
	}
	return name + "\x00" + argsJSON
}

func (t *changeTracker) put(key string, c *FileChange) {
	if c == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pending == nil {
		t.pending = make(map[string]*FileChange)
	}
	t.pending[key] = c
}

func (t *changeTracker) take(key string) *FileChange {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.pending[key]
	delete(t.pending, key)
	return c
}

// ArgsFileChange rebuilds the change a write or edit call made from its
// arguments alone, for a transcript restored from history where the file's
// state at the time is gone: an edit becomes a fragment diff of its old and
// new strings, a write shows the content it wrote. Nil for other tools.
func ArgsFileChange(name, argsJSON string) *FileChange {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Old     string `json:"old"`
		New     string `json:"new"`
	}
	if (name != "write" && name != "edit") || json.Unmarshal([]byte(argsJSON), &args) != nil {
		return nil
	}
	if name == "write" {
		return &FileChange{Path: args.Path, After: args.Content}
	}
	return &FileChange{Path: args.Path, Before: args.Old, After: args.New, Fragment: true}
}
