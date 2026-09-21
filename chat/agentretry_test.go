package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mudler/cogito"
	openai "github.com/sashabaranov/go-openai"
)

func TestAgentRetryWaitsOutRateLimit(t *testing.T) {
	waits := stubRetrySleep(t)
	r := &agentRetrier{}
	calls := 0
	got, err := retryAgentRequest(context.Background(), r, func() (string, error) {
		calls++
		if calls <= 5 {
			return "", errors.New("localai stream: status 429: " + resetAt(4*time.Minute))
		}
		return "ok", nil
	})
	if err != nil || got != "ok" {
		t.Fatalf("retryAgentRequest = %q, %v; want the sub-agent to outlast the limit", got, err)
	}
	if len(*waits) != 5 || (*waits)[0] < 3*time.Minute {
		t.Fatalf("waits = %v; want 5 waits until the reset", *waits)
	}
}

func TestAgentRetryReturnsFatalAtOnce(t *testing.T) {
	stubRetrySleep(t)
	calls := 0
	_, err := retryAgentRequest(context.Background(), &agentRetrier{}, func() (string, error) {
		calls++
		return "", errors.New("localai stream: status 401: bad key")
	})
	if err == nil || calls != 1 {
		t.Fatalf("calls = %d, err = %v; a fatal error must not be retried", calls, err)
	}
}

// After a spent budget the client tries once per call, so cogito's own
// retries do not repeat the whole budget. A success re-arms it.
func TestAgentRetryFailsFastAfterBudget(t *testing.T) {
	stubRetrySleep(t)
	r := &agentRetrier{}
	fail := func() (string, error) { return "", errors.New("status 503: busy") }

	calls := 0
	count := func(fn func() (string, error)) func() (string, error) {
		return func() (string, error) { calls++; return fn() }
	}
	if _, err := retryAgentRequest(context.Background(), r, count(fail)); err == nil {
		t.Fatal("expected the error after the budget")
	}
	if calls != turnRetryBudget {
		t.Fatalf("calls = %d, want %d", calls, turnRetryBudget)
	}

	calls = 0
	retryAgentRequest(context.Background(), r, count(fail))
	if calls != 1 {
		t.Fatalf("calls = %d after a spent budget, want 1", calls)
	}

	retryAgentRequest(context.Background(), r, func() (string, error) { return "ok", nil })
	calls = 0
	retryAgentRequest(context.Background(), r, count(fail))
	if calls != turnRetryBudget {
		t.Fatalf("calls = %d after a success, want the full budget again", calls)
	}
}

func TestAgentRetryStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	retrySleep = func(context.Context, time.Duration) error { cancel(); return context.Canceled }
	t.Cleanup(func() { retrySleep = sleepCtx })
	calls := 0
	_, err := retryAgentRequest(ctx, &agentRetrier{}, func() (string, error) {
		calls++
		return "", errors.New("status 429")
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("calls = %d, err = %v; cancelling must stop the wait", calls, err)
	}
}

type fakeStreamLLM struct{ rateLimitLLM }

func (f *fakeStreamLLM) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan cogito.StreamEvent, error) {
	return nil, nil
}

func TestRetryForAgentKeepsStreaming(t *testing.T) {
	if _, ok := retryForAgent(&fakeStreamLLM{}, nil).(cogito.StreamingLLM); !ok {
		t.Fatal("the wrapper hid streaming; cogito would fall back to blocking requests")
	}
	if _, ok := retryForAgent(&rateLimitLLM{}, nil).(cogito.StreamingLLM); ok {
		t.Fatal("the wrapper claimed streaming for a client without it")
	}
}

func TestRetryForAgentRetriesCompletions(t *testing.T) {
	stubRetrySleep(t)
	llm := &rateLimitLLM{failures: 3}
	_, _, err := retryForAgent(llm, nil).CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{})
	if err != nil || llm.calls != 4 {
		t.Fatalf("calls = %d, err = %v; want 3 retries then success", llm.calls, err)
	}
}

// A sub-agent waiting on the backend is not stalled.
func TestStaleIgnoresBackendWaits(t *testing.T) {
	s := newStaleTestSession(true)
	s.agentLogs.started("a1", time.Now().Add(-2*staleAgentAfter))

	s.agentBackoff.waiting.Add(1)
	s.notifyStaleAgents(time.Now())
	if got := drainInjected(s); len(got) != 0 {
		t.Fatalf("an agent waiting on the backend was reported: %v", got)
	}

	// The wait ended just now: idleness counts from there.
	s.agentBackoff.lastEnd.Store(time.Now().UnixNano())
	s.agentBackoff.waiting.Add(-1)
	s.notifyStaleAgents(time.Now())
	if got := drainInjected(s); len(got) != 0 {
		t.Fatalf("reported right after the wait ended: %v", got)
	}
	s.notifyStaleAgents(time.Now().Add(staleAgentAfter + time.Second))
	if got := drainInjected(s); len(got) != 1 {
		t.Fatalf("notices = %v; a real stall after the wait must still be reported", got)
	}
}
