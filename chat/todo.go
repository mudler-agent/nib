package chat

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mudler/cogito"
)

// TodoStatus is the lifecycle state of a todo item.
type TodoStatus string

const (
	TodoPending   TodoStatus = "pending"
	TodoActive    TodoStatus = "in_progress"
	TodoCompleted TodoStatus = "completed"
	TodoCancelled TodoStatus = "cancelled"
)

// ValidTodoStatus reports whether s is a recognised status value.
func ValidTodoStatus(s string) bool {
	switch TodoStatus(s) {
	case TodoPending, TodoActive, TodoCompleted, TodoCancelled:
		return true
	}
	return false
}

// TodoItem is a single task in the ephemeral todo list.
type TodoItem struct {
	Content    string     `json:"content" jsonschema:"the task description"`
	Status     TodoStatus `json:"status" jsonschema:"one of: pending, in_progress, completed, cancelled"`
	ActiveForm string     `json:"active_form,omitempty" jsonschema:"present-progressive rephrasing of the task (e.g. 'Reading files'); omit if same as content"`
}

// TodoList is the ephemeral, in-memory todo store. It is not persisted — it
// lives for the session and is cleared when the session ends. Mirrors maki's
// design: the model sends the complete list on every call (replace-all
// semantics), so there is no add/update/delete CRUD.
type TodoList struct {
	mu    sync.RWMutex
	items []TodoItem
}

// NewTodoList returns an empty ephemeral todo list.
func NewTodoList() *TodoList {
	return &TodoList{}
}

// Set replaces the entire list atomically.
func (t *TodoList) Set(items []TodoItem) {
	t.mu.Lock()
	t.items = items
	t.mu.Unlock()
}

// Items returns a snapshot copy of the current list.
func (t *TodoList) Items() []TodoItem {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]TodoItem, len(t.items))
	copy(out, t.items)
	return out
}

// Counts returns (completed, total) for footer display.
func (t *TodoList) Counts() (done, total int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	total = len(t.items)
	for _, it := range t.items {
		if it.Status == TodoCompleted {
			done++
		}
	}
	return
}

// todoWriteArgs is the argument schema for the todo_write tool.
type todoWriteArgs struct {
	Todos []TodoItem `json:"todos" jsonschema:"the complete todo list — every item, in order. Send the FULL list each time, not just changes."`
}

// todoWriteTool lets the model manage an ephemeral todo list. Each call
// replaces the entire list (replace-all semantics, like maki).
type todoWriteTool struct {
	store *TodoList
}

func (t *todoWriteTool) Run(args map[string]any) (string, any, error) {
	if t.store == nil {
		return "Todo tracking is not available in this session.", nil, nil
	}

	raw, ok := args["todos"]
	if !ok {
		return "Missing 'todos' argument.", nil, nil
	}

	rawBytes, err := json.Marshal(raw)
	if err != nil {
		return fmt.Sprintf("Failed to parse todos: %v", err), nil, nil
	}
	var items []TodoItem
	if err := json.Unmarshal(rawBytes, &items); err != nil {
		return fmt.Sprintf("Failed to parse todos: %v", err), nil, nil
	}

	// Validate statuses and normalise.
	for i := range items {
		if items[i].Status == "" {
			items[i].Status = TodoPending
		}
		if !ValidTodoStatus(string(items[i].Status)) {
			return fmt.Sprintf("Invalid status %q on item %d. Use one of: pending, in_progress, completed, cancelled.", items[i].Status, i+1), nil, nil
		}
	}

	t.store.Set(items)
	done, total := t.store.Counts()
	summary := todoSummary(items, done, total)
	return summary, nil, nil
}

// todoWriteToolDefinition builds the cogito tool definition for todo_write.
func todoWriteToolDefinition(store *TodoList) cogito.ToolDefinitionInterface {
	return cogito.NewToolDefinition[map[string]any](
		&todoWriteTool{store: store},
		todoWriteArgs{},
		"todo_write",
		"Update the todo list for the current task. Send the COMPLETE list every time — this replaces the entire list, not a delta. Use this to plan multi-step work BEFORE starting, mark items in_progress when you begin them, and completed when done. Keep exactly one item in_progress at a time. Call this at the start of any task with 3+ steps, and update it after each step completes.",
	)
}

// todoSummary renders a compact one-line summary of the list state for the tool
// result, so the model sees what it just committed.
func todoSummary(items []TodoItem, done, total int) string {
	if total == 0 {
		return "Todo list cleared."
	}
	return fmt.Sprintf("Todos updated: %d/%d completed.", done, total)
}
