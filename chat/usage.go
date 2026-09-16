package chat

import (
	"sync"

	"github.com/mudler/cogito"
)

// SessionUsage is a snapshot of what a session has spent: the LLM calls it
// made, including sub-agents and compaction summaries, plus the number of user
// turns those calls served.
//
// Every gap found so far under-reports rather than invents spend, so a figure
// here is a floor and never an overstatement. That direction is the contract;
// the list below is what is known, not a proof that nothing else leaks:
//   - Streaming. cogito reads streamed usage from StreamEvent.Usage on the done
//     event and its bundled clients never populate it, so a session that sets
//     Callbacks.OnStream counts zero. nib's CLI and TUI do not set it.
//   - A failed sub-agent. cogito keeps a sub-agent's fragment only on success,
//     so whatever a failure burned before dying has nowhere to be read from.
//   - A resumed sub-agent. send_agent_message to an agent that already finished
//     runs a fresh ExecuteTools and reassigns agent.Fragment in place, with no
//     status transition and so no agent callback: emitAgentEvent, the one place
//     sub-agent spend is folded in, never fires for that run and its tokens are
//     counted nowhere.
//
// The JSON tags are load-bearing: the usage.json written beside a trace is read
// by benchmark harnesses, so the field names are a contract, not decoration.
type SessionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	Turns            int `json:"turns"`
}

// sessionUsage accumulates SessionUsage behind its own lock.
//
// Its own lock, and not historyMu, on purpose: historyMu is held across history
// rebuilds and compaction swaps, and routing counter updates through it would
// couple the run path to those and invite lock-ordering problems for a few
// integer adds.
//
// cogito's Status.CumulativeUsage is per-RUN (see cogito/tools.go): cogito sets
// it once at the end of each ExecuteTools call, and the session reassigns its
// fragment every turn, so that field always holds the LAST turn's total and
// never the session's. Mirroring it — which was the obvious first move — would
// silently under-report every multi-turn session. This is the session total.
type sessionUsage struct {
	mu sync.Mutex
	u  SessionUsage
}

func (c *sessionUsage) add(u cogito.LLMUsage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.u.PromptTokens += u.PromptTokens
	c.u.CompletionTokens += u.CompletionTokens
	c.u.TotalTokens += u.TotalTokens
}

func (c *sessionUsage) turn() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.u.Turns++
}

func (c *sessionUsage) snapshot() SessionUsage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.u
}

// addUsage folds one LLM call's usage into the session total. Counters only
// ever grow: compaction reduces the context, never the spend.
func (s *Session) addUsage(u cogito.LLMUsage) { s.usage.add(u) }

// countTurn records one completed user exchange. Sub-agent and compaction calls
// add tokens but never turns — a turn is what the user sees, not what the
// backend was asked.
func (s *Session) countTurn() { s.usage.turn() }

// Usage returns what this session has spent so far. Safe to call from any
// goroutine, which the TUI does on every render.
func (s *Session) Usage() SessionUsage { return s.usage.snapshot() }

// EstimatedUsage derives a byte/4 estimate of the session's prompt and
// completion tokens from the current conversation (estimateUsageSplit),
// mirroring the fallback ContextTokens already applies for the context badge.
// It exists for a session whose real counter is unhelpful — chiefly a
// streamed one: cogito's bundled clients never populate StreamEvent.Usage, so
// every streamed turn adds zero to Usage() (see this file's doc comment).
//
// This method always estimates from the fragment, regardless of what Usage()
// holds; deciding whether to prefer the measured figure over this one is the
// caller's job (the TUI's usageBadge does it). Deliberately kept OFF the
// Usage() / Close() path: only Usage() feeds trace.WriteUsage's usage.json,
// so an estimate can never land under those measured field names, which the
// package doc calls a contract benchmark harnesses read. A caller that
// displays this figure must mark it as an estimate (theme.UsageEstimatedPrefix)
// rather than presenting it as measured spend.
//
// The byte/4 proxy is not guaranteed to be a floor the way Usage() is:
// content that compresses well under a real BPE tokenizer (long runs of
// whitespace or repeated characters) can make the true count lower than this
// guess. That is acceptable only because every caller marks the figure as an
// estimate rather than folding it into SessionUsage's own
// never-an-overstatement contract.
func (s *Session) EstimatedUsage() SessionUsage {
	prompt, completion := estimateUsageSplit(s.fragment.Messages)
	return SessionUsage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
		Turns:            s.Usage().Turns,
	}
}
