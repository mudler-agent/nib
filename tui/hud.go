package tui

import (
	"fmt"
	"strconv"
	"strings"
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

	// CPU: system-wide utilization between this tick and the last. The first
	// tick only records the baseline.
	if busy, total, ok := systemCPUTimes(); ok {
		if dt := total - m.hudPrevTotal; m.hudPrevTotal > 0 && total > m.hudPrevTotal {
			m.hudCPU = int((busy - m.hudPrevBusy) * 100 / dt)
			m.hudCPUOK = true
		}
		m.hudPrevBusy, m.hudPrevTotal = busy, total
	}

	// RAM: system memory in use, as free(1) counts it.
	if used, total, ok := systemMemory(); ok {
		m.hudMemUsed, m.hudMemTotal = used, total
	}
}

// initHudClock sets the initial clock value so the header isn't blank
// before the first tick.
func (m *Model) initHudClock() {
	m.hudClock = time.Now().Format("15:04:05")
}

// humanGiB formats bytes as GiB with one decimal, trimming a trailing ".0".
func humanGiB(b int64) string {
	return strings.TrimSuffix(strconv.FormatFloat(float64(b)/(1<<30), 'f', 1, 64), ".0")
}
