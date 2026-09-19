package tui

import "testing"

func TestParseCPULine(t *testing.T) {
	// user nice system idle iowait irq softirq steal guest guest_nice
	busy, total, ok := parseCPULine("cpu  100 5 50 800 40 3 2 0 7 0")
	if !ok {
		t.Fatal("a well-formed cpu line did not parse")
	}
	// guest (7) is already inside user, so it must not be counted twice.
	if total != 1000 || busy != 160 {
		t.Fatalf("busy=%d total=%d, want busy=160 total=1000", busy, total)
	}
	if _, _, ok := parseCPULine("cpu0 1 2 3 4 5"); ok {
		t.Fatal("a per-core line parsed as the aggregate")
	}
}

func TestParseMeminfo(t *testing.T) {
	used, total, ok := parseMeminfo("MemTotal:       32000000 kB\nMemFree:         1000000 kB\nMemAvailable:   20000000 kB\n")
	if !ok {
		t.Fatal("meminfo did not parse")
	}
	if total != 32000000*1024 || used != 12000000*1024 {
		t.Fatalf("used=%d total=%d", used, total)
	}
	if _, _, ok := parseMeminfo("MemTotal: 1000 kB\n"); ok {
		t.Fatal("parsed without MemAvailable; used would be the whole machine")
	}
}

// Two ticks against the real /proc give a measured CPU figure and memory.
func TestHudTickMeasuresSystem(t *testing.T) {
	var m Model
	m.handleHudTick()
	if m.hudCPUOK {
		t.Fatal("CPU reported from a single sample")
	}
	m.hudPrevTotal -= 100 // pretend time passed without waiting for jiffies
	m.handleHudTick()
	if !m.hudCPUOK || m.hudCPU < 0 || m.hudCPU > 100 {
		t.Fatalf("cpu = %d (ok=%v), want a measured 0..100", m.hudCPU, m.hudCPUOK)
	}
	if m.hudMemTotal <= 0 || m.hudMemUsed <= 0 || m.hudMemUsed > m.hudMemTotal {
		t.Fatalf("mem = %d/%d", m.hudMemUsed, m.hudMemTotal)
	}
}
