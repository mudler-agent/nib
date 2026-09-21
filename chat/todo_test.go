package chat

import (
	"testing"
)

func TestTodoListSetAndGet(t *testing.T) {
	tl := NewTodoList()
	items := []TodoItem{
		{Content: "Read files", Status: TodoCompleted},
		{Content: "Write code", Status: TodoActive},
		{Content: "Test it", Status: TodoPending},
	}
	tl.Set(items)

	got := tl.Items()
	if len(got) != 3 {
		t.Fatalf("Items() = %d items, want 3", len(got))
	}
	if got[0].Status != TodoCompleted || got[1].Status != TodoActive || got[2].Status != TodoPending {
		t.Fatalf("statuses wrong: %+v", got)
	}

	done, total := tl.Counts()
	if done != 1 || total != 3 {
		t.Fatalf("Counts() = (%d, %d), want (1, 3)", done, total)
	}
}

func TestTodoListSetReplaces(t *testing.T) {
	tl := NewTodoList()
	tl.Set([]TodoItem{{Content: "a", Status: TodoPending}})
	tl.Set([]TodoItem{{Content: "b", Status: TodoCompleted}, {Content: "c", Status: TodoCompleted}})

	got := tl.Items()
	if len(got) != 2 {
		t.Fatalf("after replace, Items() = %d, want 2", len(got))
	}
	done, total := tl.Counts()
	if done != 2 || total != 2 {
		t.Fatalf("Counts() = (%d, %d), want (2, 2)", done, total)
	}
}

func TestTodoListEmpty(t *testing.T) {
	tl := NewTodoList()
	if len(tl.Items()) != 0 {
		t.Fatalf("new list should be empty")
	}
	done, total := tl.Counts()
	if done != 0 || total != 0 {
		t.Fatalf("Counts() on empty = (%d, %d), want (0, 0)", done, total)
	}
}

func TestTodoWriteToolRun(t *testing.T) {
	store := NewTodoList()
	tool := &todoWriteTool{store: store}

	out, _, err := tool.Run(map[string]any{
		"todos": []any{
			map[string]any{
				"content": "Read files",
				"status":  "completed",
			},
			map[string]any{
				"content": "Write tests",
				"status":  "in_progress",
			},
			map[string]any{
				"content": "Deploy",
				"status":  "pending",
			},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty result")
	}

	done, total := store.Counts()
	if done != 1 || total != 3 {
		t.Fatalf("store Counts = (%d, %d), want (1, 3)", done, total)
	}
}

func TestTodoWriteToolEmptyList(t *testing.T) {
	store := NewTodoList()
	tool := &todoWriteTool{store: store}

	// Seed with items first.
	store.Set([]TodoItem{{Content: "x", Status: TodoPending}})
	if len(store.Items()) != 1 {
		t.Fatalf("seed failed")
	}

	out, _, err := tool.Run(map[string]any{
		"todos": []any{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty result for empty list")
	}
	if len(store.Items()) != 0 {
		t.Fatalf("empty todos should clear the list, got %d items", len(store.Items()))
	}
}

func TestTodoWriteToolInvalidStatus(t *testing.T) {
	store := NewTodoList()
	tool := &todoWriteTool{store: store}

	out, _, err := tool.Run(map[string]any{
		"todos": []any{
			map[string]any{
				"content": "bad",
				"status":  "bogus",
			},
		},
	})
	if err != nil {
		t.Fatalf("Run should not return error for invalid status: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty error message in result")
	}
	if len(store.Items()) != 0 {
		t.Fatalf("invalid status should not store items")
	}
}

func TestTodoWriteToolMissingTodos(t *testing.T) {
	store := NewTodoList()
	tool := &todoWriteTool{store: store}

	out, _, err := tool.Run(map[string]any{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out == "" {
		t.Fatalf("expected error message for missing todos")
	}
}

func TestTodoWriteToolNilStore(t *testing.T) {
	tool := &todoWriteTool{store: nil}
	out, _, err := tool.Run(map[string]any{
		"todos": []any{
			map[string]any{"content": "x", "status": "pending"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty result for nil store")
	}
}

func TestTodoWriteToolNormalisesEmptyStatus(t *testing.T) {
	store := NewTodoList()
	tool := &todoWriteTool{store: store}

	out, _, err := tool.Run(map[string]any{
		"todos": []any{
			map[string]any{"content": "no status"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty result")
	}
	items := store.Items()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Status != TodoPending {
		t.Fatalf("empty status should normalise to pending, got %q", items[0].Status)
	}
}

func TestValidTodoStatus(t *testing.T) {
	valid := []string{"pending", "in_progress", "completed", "cancelled"}
	for _, s := range valid {
		if !ValidTodoStatus(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	invalid := []string{"", "done", "started", "finished", "open"}
	for _, s := range invalid {
		if ValidTodoStatus(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}
