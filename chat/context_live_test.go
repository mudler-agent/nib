package chat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mudler/cogito"
	"github.com/mudler/nib/types"
	"github.com/sashabaranov/go-openai"
)

// liveUsagePrompt is the prompt-token figure liveUsageLLM reports on the
// request it makes before parking, so the assertion below is against a number
// that can only have come from THIS turn.
const liveUsagePrompt = 4242

// liveUsageLLM drives a two-call turn: the first request picks a harmless
// tool and reports a distinctive prompt-token count, the second parks — so a
// test can read the session's context size while the run is provably still in
// flight, with a figure that can only have come from THIS turn.
type liveUsageLLM struct {
	calls   atomic.Int32
	parked  chan struct{}
	release chan struct{}
}

func (l *liveUsageLLM) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (cogito.LLMReply, cogito.LLMUsage, error) {
	if err := ctx.Err(); err != nil {
		return cogito.LLMReply{}, cogito.LLMUsage{}, err
	}
	usage := cogito.LLMUsage{PromptTokens: liveUsagePrompt, CompletionTokens: 8, TotalTokens: liveUsagePrompt + 8}
	if l.calls.Add(1) == 1 {
		return cogito.LLMReply{
			ChatCompletionResponse: openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{{
					Message: openai.ChatCompletionMessage{
						Role: "assistant",
						ToolCalls: []openai.ToolCall{{
							ID:       "call-1",
							Type:     openai.ToolTypeFunction,
							Function: openai.FunctionCall{Name: "agent_logs", Arguments: `{"agent_id":"none"}`},
						}},
					},
					FinishReason: openai.FinishReasonToolCalls,
				}},
			},
		}, usage, nil
	}
	// Second request: the first one's usage is now on the run's Status. Park
	// here so the assertion happens with the turn unfinished.
	select {
	case l.parked <- struct{}{}:
	default:
	}
	<-l.release
	return cogito.LLMReply{
		ChatCompletionResponse: openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{{
				Message:      openai.ChatCompletionMessage{Role: "assistant", Content: "ok"},
				FinishReason: openai.FinishReasonStop,
			}},
		},
	}, usage, nil
}

func (l *liveUsageLLM) Ask(ctx context.Context, f cogito.Fragment) (cogito.Fragment, error) {
	return f.AddMessage("assistant", "ok"), nil
}

// TestContextTokensTracksTheRunInFlight is the stale context-gauge bug: the
// footer's "used" figure froze for the whole turn and only jumped at the end.
//
// SendMessage hands cogito a DEEP COPY of the fragment's Status (so a failed
// run cannot pollute the session's own PastActions/ToolsCalled), and reassigns
// s.fragment only once ExecuteTools has returned. cogito updates LastUsage on
// that copy after every LLM call in the tool loop, but the session kept no
// reference to it — so ContextTokens, which reads s.fragment, reported the
// PREVIOUS turn's prompt size for the entire duration of the current one. A
// multi-step turn could add fifty thousand tokens with the gauge unmoved.
func TestContextTokensTracksTheRunInFlight(t *testing.T) {
	llm := &liveUsageLLM{parked: make(chan struct{}, 1), release: make(chan struct{})}
	s := &Session{
		ctx:           context.Background(),
		llm:           llm,
		systemPrompt:  "You are the live-context test assistant.",
		fragment:      cogito.NewEmptyFragment(),
		cogitoOptions: types.AgentOptions{Iterations: 10, MaxAttempts: 3, MaxRetries: 3},
		agentManager:  cogito.NewAgentManager(),
		agentLogs:     newAgentLogStore(),
		inject:        make(chan openai.ChatCompletionMessage, 8),
		toolAllow:     map[string]bool{"agent_logs": true},
	}

	done := make(chan error, 1)
	go func() {
		_, err := s.SendMessage("hello")
		done <- err
	}()

	select {
	case <-llm.parked:
	case <-time.After(5 * time.Second):
		t.Fatal("the turn never reached the LLM")
	}

	got := s.ContextTokens()
	close(llm.release)

	if err := <-done; err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if got != liveUsagePrompt {
		t.Fatalf("ContextTokens() mid-turn = %d, want %d (the gauge is reading the previous turn's fragment)", got, liveUsagePrompt)
	}
}
