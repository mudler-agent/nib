package tui

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// feed records n one-token chunks, one every gap, from start; it returns the
// time of the last one.
func feed(s *speedMeter, start time.Time, n int, gap time.Duration) time.Time {
	at := start
	for i := 0; i < n; i++ {
		at = start.Add(time.Duration(i) * gap)
		s.record(3, at) // under 4 bytes: one token each
	}
	return at
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > want*0.05 {
		t.Errorf("%s = %.2f, want about %.2f", name, got, want)
	}
}

// While chunks arrive, the reading is live at the rate they arrive.
func TestSpeedLiveRate(t *testing.T) {
	var s speedMeter
	t0 := time.Now()
	last := feed(&s, t0, 101, 20*time.Millisecond) // 50 tok/s for 2s
	r, ok := s.read(last)
	if !ok || !r.Live {
		t.Fatalf("reading = %+v, ok %v; want live", r, ok)
	}
	near(t, "live rate", r.Rate, 50)
	near(t, "average", r.Avg, 50)
	if len(r.Spark) != speedBuckets {
		t.Fatalf("spark has %d bars, want %d", len(r.Spark), speedBuckets)
	}
}

// A pause longer than speedGap (a tool call, the next request waiting for
// its first token) is not generation time: the average stays the decode
// speed, and idle the reading shows the last stretch's rate.
func TestSpeedLeavesPausesOut(t *testing.T) {
	var s speedMeter
	t0 := time.Now()
	end1 := feed(&s, t0, 51, 20*time.Millisecond) // 50 tok/s for 1s
	start2 := end1.Add(10 * time.Second)
	end2 := feed(&s, start2, 26, 40*time.Millisecond) // 25 tok/s for 1s

	r, _ := s.read(end2.Add(5 * time.Second))
	if r.Live {
		t.Fatal("reading still live 5s after the last chunk")
	}
	near(t, "last rate", r.Rate, 25)
	// 77 tokens over 2s of generation, not over the 12s of wall time.
	near(t, "average", r.Avg, 77.0/2)
}

// A chunk of many bytes counts as bytes/4 tokens, so a provider that sends
// several tokens per chunk is not measured as one token per chunk.
func TestSpeedCountsLongChunksByBytes(t *testing.T) {
	var s speedMeter
	t0 := time.Now()
	for i := 0; i <= 10; i++ {
		s.record(40, t0.Add(time.Duration(i)*100*time.Millisecond)) // 10 tokens each
	}
	r, _ := s.read(t0.Add(time.Second))
	near(t, "average", r.Avg, 110)
}

func TestSpeedBadges(t *testing.T) {
	m := Model{speed: &speedMeter{}, loading: true}
	if full, narrow := m.speedBadges(); full != "" || narrow != "" || m.liveSpeed() != "" {
		t.Fatal("speed shown before any chunk")
	}
	feed(m.speed, time.Now().Add(-time.Second), 51, 20*time.Millisecond)
	full, narrow := m.speedBadges()
	if got := ansi.Strip(full); !strings.HasPrefix(got, "tok/s ") || !strings.Contains(got, " avg ") {
		t.Fatalf("full badge = %q, want the rate and the average", got)
	}
	if got := ansi.Strip(narrow); !strings.HasPrefix(got, "avg ") || !strings.HasSuffix(got, " tok/s") {
		t.Fatalf("narrow badge = %q, want the average alone", got)
	}
	if got := ansi.Strip(m.liveSpeed()); !strings.Contains(got, " tok/s") {
		t.Fatalf("live speed = %q, want the live rate", got)
	}
	m.loading = false
	if got := m.liveSpeed(); got != "" {
		t.Fatalf("live speed shown with no turn running: %q", got)
	}
}

func TestFormatRate(t *testing.T) {
	for r, want := range map[float64]string{0.46: "0.5", 7.25: "7.2", 42.4: "42", 131.6: "132"} {
		if got := formatRate(r); got != want {
			t.Errorf("formatRate(%v) = %q, want %q", r, got, want)
		}
	}
}
