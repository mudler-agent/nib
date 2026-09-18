package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/mudler/nib/theme"
)

// bootEntry is one line in the boot log.
type bootEntry struct {
	ts    string // elapsed timestamp, e.g. "00.041"
	ev    string // event name, e.g. "config"
	dt    string // detail, e.g. "~/.config/nib/config.yaml"
	ready bool   // marks the final READY line
}

// bootState tracks the boot log animation. It is a live state, not a gate:
// the user can type immediately and messages queue until READY. The log
// stays visible until the first message is sent, then collapses.
type bootState struct {
	entries   []bootEntry // accumulated lines so far
	index     int         // next scripted event to emit
	done      bool        // sessionReadyMsg received
	collapsed bool        // first message sent; log hidden
	start     time.Time
}

func newBootState() *bootState {
	return &bootState{start: time.Now()}
}

// bootScriptEntry is one scripted boot event with a target delay (ms from start).
type bootScriptEntry struct {
	delayMs int
	ev      string
	dt      string
}

func bootScript() []bootScriptEntry {
	return []bootScriptEntry{
		{0,   "core/init",    "nib :: starting"},
		{60,  "config",       ""},
		{120, "provider",     ""},
		{200, "model",        ""},
		{260, "mcp.connect",  ""},
		{400, "tools.mount",  ""},
		{460, "skills.index", ""},
		{520, "session",      ""},
		{580, "memory",       "0 notes loaded"},
	}
}

// bootTickMsg advances the boot animation by emitting the next scripted entry.
type bootTickMsg struct{}

// nextBootCmd returns a command that emits the next boot entry after its delay.
func (b *bootState) nextBootCmd() tea.Cmd {
	if b.index >= len(bootScript()) {
		return nil
	}
	entries := bootScript()
	entry := entries[b.index]
	target := time.Duration(entry.delayMs) * time.Millisecond

	elapsed := time.Since(b.start)
	delay := target - elapsed
	if delay < 0 {
		delay = 0
	}

	return tea.Tick(delay, func(time.Time) tea.Msg {
		return bootTickMsg{}
	})
}

// tick appends the next scripted entry, filling in real data where available.
func (b *bootState) tick(m *Model) {
	if b.index >= len(bootScript()) {
		return
	}
	entries := bootScript()
	e := entries[b.index]
	b.index++

	entry := bootEntry{
		ts: fmt.Sprintf("%05.3f", time.Since(b.start).Seconds()),
		ev: e.ev,
	}
	if e.dt != "" {
		entry.dt = e.dt
	}

	switch e.ev {
	case "config":
		if m.cfg.BaseDir != "" {
			entry.dt = m.cfg.BaseDir
		} else {
			entry.dt = "defaults"
		}
	case "provider":
		entry.dt = m.cfg.Provider
		if entry.dt == "" {
			entry.dt = "openai-compat"
		}
	case "model":
		model := m.cfg.Model
		if model == "" {
			model = "default"
		}
		entry.dt = model
	case "mcp.connect":
		n := len(m.transports)
		if n == 0 {
			entry.dt = "no transports"
		} else {
			entry.dt = fmt.Sprintf("connecting %d transports", n)
		}
	case "tools.mount":
		entry.dt = "tools registered"
	case "skills.index":
		n := len(m.cfg.Skills)
		entry.dt = fmt.Sprintf("%d skills indexed", n)
	case "session":
		sid := m.sessionID
		if len(sid) > 8 {
			sid = sid[:8]
		}
		entry.dt = fmt.Sprintf("new :: %s", sid)
	}

	b.entries = append(b.entries, entry)
}

// markReady flushes remaining scripted entries and appends the READY line.
func (b *bootState) markReady() {
	script := bootScript()
	for b.index < len(script) {
		e := script[b.index]
		b.index++
		b.entries = append(b.entries, bootEntry{
			ts: fmt.Sprintf("%05.3f", time.Since(b.start).Seconds()),
			ev: e.ev,
			dt: e.dt,
		})
	}
	b.entries = append(b.entries, bootEntry{
		ts:    fmt.Sprintf("%05.3f", time.Since(b.start).Seconds()),
		ready: true,
	})
	b.done = true
}

// render produces the boot log string for the body area.
func (b *bootState) render(width int) string {
	if b == nil || b.collapsed {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n")
	for _, e := range b.entries {
		if e.ready {
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				theme.Meta.Render("["+e.ts+"]"),
				theme.Done.Render("✓ READY")))
			continue
		}
		ev := e.ev
		if len(ev) < 13 {
			ev = ev + strings.Repeat(" ", 13-len(ev))
		}
		sb.WriteString(fmt.Sprintf("  %s  %s %s%s\n",
			theme.Meta.Render("["+e.ts+"]"),
			theme.Done.Render("✓"),
			theme.Running.Render(ev),
			theme.Help.Render(e.dt),
		))
	}
	return sb.String()
}
