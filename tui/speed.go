package tui

import (
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/mudler/nib/theme"
)

// speedMeter measures how fast the model generates, from the stream itself.
//
// It cannot use measured usage: cogito's bundled clients never populate
// StreamEvent.Usage, so a streamed turn reports no completion tokens (see
// chat/usage.go). Each streamed chunk (reasoning or content) is counted as
// max(1, bytes/4) tokens instead. Most local servers send one token per
// chunk, which the 1 counts exactly; a provider that sends several tokens per
// chunk is caught by the bytes/4 estimate the context badge also uses.
//
// Only generation time counts. A stretch runs from a chunk to the next one
// while they come less than speedGap apart, so waiting for the first token
// (the prompt being read) and running tools are left out: the figure is the
// model's decode speed, which is what changes between models and machines.
//
// OnStream records on the session's goroutine and the footer reads on the UI
// goroutine, so the meter has its own lock.
type speedMeter struct {
	mu sync.Mutex

	// The stretch in progress: from its first chunk (start) to its latest
	// (last), with tokens generated in it.
	active      bool
	start, last time.Time
	tokens      float64

	// Stretches that have ended, for the session average.
	doneTokens float64
	doneTime   time.Duration
	// lastRate is the rate of the stretch that ended last, shown when the
	// model is idle.
	lastRate float64

	// buckets hold tokens per speedBucket, the newest at the end, for the
	// live rate and the sparkline. bucketAt is when the newest one started.
	buckets  [speedBuckets]float64
	bucketAt time.Time
}

const (
	// speedGap ends a stretch: a pause this long is a tool call or a new
	// request, not the model generating.
	speedGap = 2 * time.Second
	// speedBucket and speedBuckets size the live window: the live rate is
	// over the last two seconds, and the sparkline shows each bucket.
	speedBucket  = 250 * time.Millisecond
	speedBuckets = 8
)

// record counts one streamed chunk of n bytes, received at now.
func (s *speedMeter) record(n int, now time.Time) {
	if n <= 0 {
		return
	}
	tokens := math.Max(1, float64(n)/4)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active && now.Sub(s.last) > speedGap {
		s.closeLocked()
	}
	if !s.active {
		s.active, s.start, s.tokens = true, now, 0
		s.buckets, s.bucketAt = [speedBuckets]float64{}, now
	}
	s.last = now
	s.tokens += tokens
	s.shiftLocked(now)
	s.buckets[speedBuckets-1] += tokens
}

// closeLocked ends the stretch in progress and adds it to the session.
func (s *speedMeter) closeLocked() {
	if d := s.last.Sub(s.start); d > 0 {
		s.doneTokens += s.tokens
		s.doneTime += d
		s.lastRate = s.stretchRate()
	}
	s.active = false
}

// stretchRate is the decode rate of the stretch in progress. Its first
// token arrives at start, so it does not count toward the time after it.
func (s *speedMeter) stretchRate() float64 {
	d := s.last.Sub(s.start).Seconds()
	if d <= 0 {
		return 0
	}
	return math.Max(0, s.tokens-1) / d
}

// shiftLocked moves the bucket window forward so its newest bucket holds now.
func (s *speedMeter) shiftLocked(now time.Time) {
	steps := int(now.Sub(s.bucketAt) / speedBucket)
	if steps <= 0 {
		return
	}
	if steps >= speedBuckets {
		s.buckets = [speedBuckets]float64{}
	} else {
		copy(s.buckets[:], s.buckets[steps:])
		for i := speedBuckets - steps; i < speedBuckets; i++ {
			s.buckets[i] = 0
		}
	}
	s.bucketAt = s.bucketAt.Add(time.Duration(steps) * speedBucket)
}

// speedReading is what the footer shows.
type speedReading struct {
	// Live is true while the model is generating: Rate is then the rate over
	// the last two seconds and Spark the tokens per second of each bucket.
	// Idle, Rate is the rate of the last stretch.
	Live  bool
	Rate  float64
	Avg   float64
	Spark []float64
}

// read returns the meter's reading at now; ok is false before any chunk.
func (s *speedMeter) read(now time.Time) (r speedReading, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active && now.Sub(s.last) > speedGap {
		s.closeLocked()
	}
	tokens, dur := s.doneTokens, s.doneTime
	if s.active {
		tokens += s.tokens
		dur += s.last.Sub(s.start)
	}
	if tokens == 0 {
		return r, false
	}
	if dur > 0 {
		r.Avg = tokens / dur.Seconds()
	}
	if !s.active {
		r.Rate = s.lastRate
		return r, true
	}
	r.Live = true
	s.shiftLocked(now)
	// The buckets cover the seven before the newest one plus the part of the
	// newest that has passed. And the window cannot be longer than the
	// stretch: a stretch 500ms old has half a second of tokens, not two.
	covered := now.Sub(s.bucketAt) + (speedBuckets-1)*speedBucket
	window := min(covered, now.Sub(s.start))
	var sum float64
	for _, b := range s.buckets {
		sum += b
	}
	if window > 0 {
		r.Rate = sum / window.Seconds()
	} else {
		r.Rate = s.stretchRate()
	}
	r.Spark = make([]float64, speedBuckets)
	for i, b := range s.buckets {
		r.Spark[i] = b / speedBucket.Seconds()
	}
	return r, true
}

// liveSpeed renders the rate for the working indicator line, "42 tok/s ▃▅▆▇",
// while the model is generating; "" otherwise. It sits where the user looks
// while the model works, and needs no room in the footer.
func (m Model) liveSpeed() string {
	if m.speed == nil || !m.loading {
		return ""
	}
	r, ok := m.speed.read(time.Now())
	if !ok || !r.Live {
		return ""
	}
	out := theme.Running.Render(formatRate(r.Rate)) + theme.Help.Render(" tok/s")
	if spark := theme.Sparkline(r.Spark); spark != "" {
		out += " " + theme.Running.Render(spark)
	}
	return out
}

// speedBadges renders the footer's speed badge in its full form, "tok/s 41 ·
// avg 38" (the rate now, or of the last reply when idle, and the session
// average), and its narrow form, "avg 38 tok/s". Both are "" before the model
// has generated anything.
func (m Model) speedBadges() (full, narrow string) {
	if m.speed == nil {
		return "", ""
	}
	r, ok := m.speed.read(time.Now())
	if !ok || r.Avg <= 0 {
		return "", ""
	}
	avg := theme.Meta.Render(formatRate(r.Avg))
	full = theme.Help.Render("tok/s ") + theme.Meta.Render(formatRate(r.Rate)) +
		theme.Help.Render(" "+theme.Sep+" avg ") + avg
	narrow = theme.Help.Render("avg ") + avg + theme.Help.Render(" tok/s")
	return full, narrow
}

// formatRate formats a rate in tokens per second: whole numbers, with one
// decimal below 10 so a slow model does not read as 0 or 1.
func formatRate(r float64) string {
	if r < 10 {
		return strconv.FormatFloat(r, 'f', 1, 64)
	}
	return strconv.Itoa(int(math.Round(r)))
}
