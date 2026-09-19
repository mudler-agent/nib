package chat

import (
	"context"
	"sync/atomic"

	"github.com/mudler/cogito"
	"github.com/sashabaranov/go-openai"
)

// liveUsage records the prompt-token count reported by the most recent request
// of the turn in flight, so ContextTokens can answer "how full is the context
// RIGHT NOW" instead of "how full was it when the last turn ended".
//
// It exists because the session cannot read the running turn's own figure.
// SendMessage hands cogito a deep copy of Fragment.Status (so a failed run
// cannot pollute the session's PastActions/ToolsCalled — see the copy in
// SendMessage) and reassigns s.fragment only once ExecuteTools has returned.
// cogito does update LastUsage on that copy after every call in the tool loop,
// but reading it from the UI goroutine while the turn goroutine writes it is a
// data race, and cogito offers no per-call usage callback to subscribe to.
//
// So the figure is taken one layer lower, from the LLM client itself: every
// request a turn makes passes through the wrapper below, on the turn
// goroutine, and stores its prompt tokens here. A single int64 is enough —
// prompt tokens are the whole conversation as the backend counted it, not a
// delta to accumulate.
//
// active distinguishes "no turn is running" from "a turn is running and has
// not been billed yet". Without it a zero would be indistinguishable from an
// idle session and ContextTokens could not tell which source to trust.
type liveUsage struct {
	prompt atomic.Int64
	active atomic.Bool
}

// begin arms the tracker for a new turn, discarding the previous turn's
// figure: the fragment is the authority between turns.
func (l *liveUsage) begin() {
	l.prompt.Store(0)
	l.active.Store(true)
}

// end disarms it, handing authority back to s.fragment — which by then holds
// the completed run's Status, and which compaction may since have shrunk.
func (l *liveUsage) end() {
	l.active.Store(false)
	l.prompt.Store(0)
}

// reset drops the in-flight figure without disarming: compaction replaced the
// conversation the last request was measured against, so that number now
// describes history nobody is sending any more. ContextTokens falls back to
// the rebuilt fragment's estimate until the next request reports a real one.
func (l *liveUsage) reset() {
	l.prompt.Store(0)
}

// record stores a request's prompt tokens. Zero and negative counts are
// ignored: a backend that reports no usage must not erase the last real
// figure and drop the gauge back to the estimate mid-turn.
func (l *liveUsage) record(n int) {
	if n > 0 {
		l.prompt.Store(int64(n))
	}
}

// promptTokens returns the in-flight figure, or 0 when no turn is running or
// none of its requests has reported usage yet.
func (l *liveUsage) promptTokens() int {
	if !l.active.Load() {
		return 0
	}
	return int(l.prompt.Load())
}

// trackedLLM is the LLM the turn loop actually calls: the session's client,
// with every request's reported usage recorded on the way back.
type trackedLLM struct {
	cogito.LLM
	live *liveUsage
}

func (t *trackedLLM) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (cogito.LLMReply, cogito.LLMUsage, error) {
	reply, usage, err := t.LLM.CreateChatCompletion(ctx, request)
	t.live.record(usage.PromptTokens)
	return reply, usage, err
}

func (t *trackedLLM) Ask(ctx context.Context, f cogito.Fragment) (cogito.Fragment, error) {
	out, err := t.LLM.Ask(ctx, f)
	if out.Status != nil {
		t.live.record(out.Status.LastUsage.PromptTokens)
	}
	return out, err
}

// trackedStreamingLLM is trackedLLM for a client that streams. The extra type
// is not optional: cogito picks the streaming path with a type assertion
// (`llm.(StreamingLLM)`), so a wrapper that did not also implement
// CreateChatCompletionStream would silently switch every streaming provider
// back to blocking requests — the thinking box and the reply would stop
// filling progressively.
type trackedStreamingLLM struct {
	trackedLLM
	stream cogito.StreamingLLM
}

// CreateChatCompletionStream relays the provider's events unchanged and reads
// the usage off the terminal "done" event, which is the only place a streamed
// request reports it.
//
// The relay goroutine is bounded by the source channel: it ends when the
// provider closes it, which cogito's own consumer loop already depends on.
func (t *trackedStreamingLLM) CreateChatCompletionStream(ctx context.Context, request openai.ChatCompletionRequest) (<-chan cogito.StreamEvent, error) {
	src, err := t.stream.CreateChatCompletionStream(ctx, request)
	if err != nil {
		return src, err
	}
	out := make(chan cogito.StreamEvent)
	go func() {
		defer close(out)
		for ev := range src {
			if ev.Type == cogito.StreamEventDone {
				t.live.record(ev.Usage.PromptTokens)
			}
			out <- ev
		}
	}()
	return out, nil
}

// trackUsage wraps llm so the turn's requests report their size to live,
// preserving streaming support when the client has it.
func trackUsage(llm cogito.LLM, live *liveUsage) cogito.LLM {
	if s, ok := llm.(cogito.StreamingLLM); ok {
		return &trackedStreamingLLM{trackedLLM: trackedLLM{LLM: llm, live: live}, stream: s}
	}
	return &trackedLLM{LLM: llm, live: live}
}
