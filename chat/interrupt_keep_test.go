package chat

import (
	"testing"

	"github.com/mudler/cogito"
	openai "github.com/sashabaranov/go-openai"
)

func TestKeepFailedTurn(t *testing.T) {
	committed := cogito.NewEmptyFragment().AddMessage("user", "list files")
	call := openai.ChatCompletionMessage{
		Role:      "assistant",
		ToolCalls: []openai.ToolCall{{ID: "1", Function: openai.FunctionCall{Name: "bash"}}},
	}
	result := openai.ChatCompletionMessage{Role: "tool", Content: "a b", ToolCallID: "1"}
	pending := openai.ChatCompletionMessage{
		Role:      "assistant",
		ToolCalls: []openai.ToolCall{{ID: "2", Function: openai.FunctionCall{Name: "bash"}}},
	}

	with := func(extra ...openai.ChatCompletionMessage) cogito.Fragment {
		f := committed
		f.Messages = append(append([]openai.ChatCompletionMessage{}, committed.Messages...), extra...)
		return f
	}

	tests := []struct {
		name     string
		returned cogito.Fragment
		want     []string
	}{
		{"no progress", committed, []string{"user", "user"}},
		{"empty fragment", cogito.Fragment{}, []string{"user", "user"}},
		{"finished tool step", with(call, result), []string{"user", "assistant", "tool", "user"}},
		{"tool call without result", with(call, result, pending), []string{"user", "assistant", "tool", "user"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keepFailedTurn(committed, tt.returned, interruptedTurnNote)
			var roles []string
			for _, m := range got.Messages {
				roles = append(roles, m.Role)
			}
			if len(roles) != len(tt.want) {
				t.Fatalf("roles = %v, want %v", roles, tt.want)
			}
			for i := range roles {
				if roles[i] != tt.want[i] {
					t.Fatalf("roles = %v, want %v", roles, tt.want)
				}
			}
			if last := got.Messages[len(got.Messages)-1]; last.Content != interruptedTurnNote {
				t.Errorf("last message = %q, want the interrupt note", last.Content)
			}
			if got.Status != committed.Status {
				t.Error("kept fragment must carry the committed Status, not the run's")
			}
		})
	}
}
