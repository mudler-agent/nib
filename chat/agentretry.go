package chat

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mudler/cogito"
	"github.com/mudler/xlog"
	"github.com/sashabaranov/go-openai"
)

// Sub-agents have no SendMessage loop to wait out a long rate limit, so their
// client does the long waits itself (see retry.go for the classes). A
// sub-agent runs in the background, so blocking its request is the right
// place to wait.
//
// cogito still retries every call that fails, so after turnRetryBudget
// failed attempts in a row the client fails fast (one try per call) until a
// request succeeds. Otherwise cogito's retries would repeat the whole budget.

// agentBackoff is shared by the sub-agent clients of one session. It records
// when a sub-agent last waited on the backend, so the stall detector does not
// report an agent that is only waiting out a rate limit.
type agentBackoff struct {
	waiting atomic.Int32
	lastEnd atomic.Int64 // UnixNano of the last wait's end
}

// quietSince returns the moment from which idleness counts: now while a wait
// is in progress, else the end of the last wait (zero if none).
func (b *agentBackoff) quietSince(now time.Time) time.Time {
	if b == nil {
		return time.Time{}
	}
	if b.waiting.Load() > 0 {
		return now
	}
	if n := b.lastEnd.Load(); n > 0 {
		return time.Unix(0, n)
	}
	return time.Time{}
}

// agentRetrier holds the retry state of one sub-agent client.
type agentRetrier struct {
	backoff *agentBackoff

	mu        sync.Mutex
	exhausted bool // the last attempt spent the whole budget
}

func (r *agentRetrier) setExhausted(v bool) {
	r.mu.Lock()
	r.exhausted = v
	r.mu.Unlock()
}

func (r *agentRetrier) isExhausted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.exhausted
}

// wait sleeps for d, or until ctx is done, while telling the stall detector.
func (r *agentRetrier) wait(ctx context.Context, d time.Duration) error {
	if r.backoff != nil {
		r.backoff.waiting.Add(1)
		defer func() {
			r.backoff.lastEnd.Store(time.Now().UnixNano())
			r.backoff.waiting.Add(-1)
		}()
	}
	return retrySleep(ctx, d)
}

// retryAgentRequest calls fn until it succeeds, fails with a fatal error, or
// fails turnRetryBudget times in a row, waiting between attempts like the
// session layer does. Cancelling ctx stops it.
func retryAgentRequest[T any](ctx context.Context, r *agentRetrier, fn func() (T, error)) (T, error) {
	var zero T
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		result, err := fn()
		if err == nil {
			r.setExhausted(false)
			return result, nil
		}
		if classifyBackendError(err) == errFatal || r.isExhausted() {
			return result, err
		}
		if attempt+1 >= turnRetryBudget {
			r.setExhausted(true)
			return result, err
		}
		wait := turnWait(err, attempt)
		xlog.Warn("sub-agent backend error, retrying", "error", err, "wait", wait, "attempt", attempt+1)
		if serr := r.wait(ctx, wait); serr != nil {
			return zero, serr
		}
	}
}

// retryingAgentLLM is a sub-agent client that retries as described above.
type retryingAgentLLM struct {
	cogito.LLM
	r *agentRetrier
}

func (a *retryingAgentLLM) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (cogito.LLMReply, cogito.LLMUsage, error) {
	var usage cogito.LLMUsage
	reply, err := retryAgentRequest(ctx, a.r, func() (cogito.LLMReply, error) {
		r, u, e := a.LLM.CreateChatCompletion(ctx, request)
		usage = u
		return r, e
	})
	return reply, usage, err
}

func (a *retryingAgentLLM) Ask(ctx context.Context, f cogito.Fragment) (cogito.Fragment, error) {
	return retryAgentRequest(ctx, a.r, func() (cogito.Fragment, error) {
		return a.LLM.Ask(ctx, f)
	})
}

// retryingAgentStreamingLLM keeps streaming: cogito picks the streaming path
// with a type assertion, so the wrapper must implement it too.
type retryingAgentStreamingLLM struct {
	retryingAgentLLM
	stream cogito.StreamingLLM
}

func (a *retryingAgentStreamingLLM) CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan cogito.StreamEvent, error) {
	return retryAgentRequest(ctx, a.r, func() (<-chan cogito.StreamEvent, error) {
		return a.stream.CreateChatCompletionStream(ctx, request)
	})
}

// retryForAgent wraps llm as a sub-agent client, keeping streaming support.
func retryForAgent(llm cogito.LLM, backoff *agentBackoff) cogito.LLM {
	r := &agentRetrier{backoff: backoff}
	if s, ok := llm.(cogito.StreamingLLM); ok {
		return &retryingAgentStreamingLLM{retryingAgentLLM: retryingAgentLLM{LLM: llm, r: r}, stream: s}
	}
	return &retryingAgentLLM{LLM: llm, r: r}
}
