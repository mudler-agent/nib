package tui

import (
	"fmt"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbletea"
)

// headerModel returns the model name for the header bar.
func (m Model) headerModel() string {
	if m.session != nil {
		if name := m.session.Model(); name != "" {
			return name
		}
	}
	if m.cfg.Model != "" {
		return m.cfg.Model
	}
	return ""
}

// headerCtx returns the context size badge for the header bar.
func (m Model) headerCtx() string {
	if m.contextTokens > 0 {
		return m.contextBadge()
	}
	return ""
}

// headerToolCount returns the number of registered tools.
func (m Model) headerToolCount() int {
	if m.session != nil {
		return m.session.ToolCount()
	}
	return 0
}

// headerMCP returns the MCP connection status string.
func (m Model) headerMCP() string {
	n := len(m.transports)
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d mcp", n)
}

// hudTickMsg drives the clock + CPU/RAM refresh.
type hudTickMsg struct{}

func (m Model) hudTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return hudTickMsg{}
	})
}

func (m *Model) handleHudTick() {
	now := time.Now()
	m.hudClock = now.Format("15:04:05")

	// CPU: count goroutines as a rough activity proxy. Not a real CPU%,
	// but gives the header a live pulse without a cgo dependency.
	// Scale: 1 goroutine ≈ 1%, capped at 100.
	goroutines := runtime.NumGoroutine()
	m.hudCPU = goroutines
	if m.hudCPU > 100 {
		m.hudCPU = 100
	}

	// RAM: use runtime memory stats (Sys in MB).
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m.hudRAM = int(ms.Sys / 1024 / 1024)
}

// initHudClock sets the initial clock value so the header isn't blank
// before the first tick.
func (m *Model) initHudClock() {
	m.hudClock = time.Now().Format("15:04:05")
}
