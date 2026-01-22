// package goose provides a simple load generator and stats collection/histogram helpers.
package goose

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// -------------------- package-global statistics --------------------

var (
	statsMu     sync.Mutex
	sendTimes   map[int]time.Time // map[ClientID] -> send time for matching replies
	samples     []time.Duration   // recorded response times (for histogram & quantiles)
	attempts    int               // number of send attempts (including skipped)
	sent        int               // number of successful sends
	skipped     int               // attempts skipped because reqCh would block
	received    int               // number of replies processed
	initialized bool              // whether ResetStats has been called
)

// ResetStats initializes or clears the package statistics. Call before a new experiment.
func ResetStats() {
	statsMu.Lock()
	defer statsMu.Unlock()
	sendTimes = make(map[int]time.Time)
	samples = make([]time.Duration, 0, 1024)
	attempts = 0
	sent = 0
	skipped = 0
	received = 0
	initialized = true
}

// internal ensure initialization
func ensureInitLocked() {
	if !initialized {
		sendTimes = make(map[int]time.Time)
		samples = make([]time.Duration, 0, 1024)
		initialized = true
	}
}

// SendUpcall records an attempted send. If skipped==true, the attempt failed and is counted as skipped.
// If skipped==false, we record the send timestamp so a later ReceiveUpcall can compute response time.
func SendUpcall(r Request, skippedFlag bool) {
	statsMu.Lock()
	defer statsMu.Unlock()
	ensureInitLocked()
	attempts++
	if skippedFlag {
		skipped++
		return
	}
	// record send
	sent++
	sendTimes[r.ClientID] = time.Now()
}

// ReceiveUpcall processes an arrived reply: it matches to a send time and records the response duration.
// If no matching send exists (e.g., we skipped that request), the reply is ignored.
func ReceiveUpcall(r Request) {
	statsMu.Lock()
	defer statsMu.Unlock()
	ensureInitLocked()
	start, ok := sendTimes[r.ClientID]
	if !ok {
		// reply for unknown clientID -> ignore
		return
	}
	rt := time.Since(start)
	samples = append(samples, rt)
	received++
	delete(sendTimes, r.ClientID)
}

// GetStats returns summary counters and mean response time in milliseconds.
func GetStats() (attemptsOut, sentOut, skippedOut, receivedOut int, meanRTms float64) {
	statsMu.Lock()
	defer statsMu.Unlock()
	ensureInitLocked()
	attemptsOut = attempts
	sentOut = sent
	skippedOut = skipped
	receivedOut = received
	if len(samples) == 0 {
		meanRTms = 0
	} else {
		var sum time.Duration
		for _, d := range samples {
			sum += d
		}
		meanRTms = float64(sum.Milliseconds()) / float64(len(samples))
	}
	return
}

// GetSamples returns a copy of recorded response-time samples (durations).
func GetSamples() []time.Duration {
	statsMu.Lock()
	defer statsMu.Unlock()
	ensureInitLocked()
	out := make([]time.Duration, len(samples))
	copy(out, samples)
	return out
}

// -------------------- histogram helpers --------------------

// HistogramLinear computes counts for linear bins over [0, maxMs).
// bins is the number of bins (excluding the overflow bin). The returned counts
// slice has length bins+1, where the last entry counts samples >= maxMs.
// Example: bins=10, maxMs=100 -> 10 bins each width=10ms and a final overflow bin >=100ms.
func HistogramLinear(bins int, maxMs float64) (counts []int, labels []string) {
	if bins <= 0 {
		bins = 10
	}
	statsMu.Lock()
	samps := make([]time.Duration, len(samples))
	copy(samps, samples)
	statsMu.Unlock()

	counts = make([]int, bins+1)
	labels = make([]string, bins+1)
	// prepare labels
	width := maxMs / float64(bins)
	for i := 0; i < bins; i++ {
		low := width * float64(i)
		high := width * float64(i+1)
		labels[i] = fmt.Sprintf("%.0f-%.0fms", low, high)
	}
	labels[bins] = fmt.Sprintf("%.0fms+", maxMs)

	// bin samples
	if len(samps) == 0 {
		return counts, labels
	}
	for _, d := range samps {
		ms := float64(d.Microseconds()) / 1000.0
		if ms < 0 {
			ms = 0
		}
		if ms >= maxMs {
			counts[bins]++
		} else {
			idx := int(ms / width)
			if idx < 0 {
				idx = 0
			} else if idx >= bins {
				idx = bins - 1
			}
			counts[idx]++
		}
	}
	return counts, labels
}

// PrintHistogramASCII prints a simple ASCII horizontal bar chart for counts with given labels.
// width controls the maximum bar length in characters.
func PrintHistogramASCII(counts []int, labels []string, width int) {
	if width <= 0 {
		width = 50
	}
	// find max count
	maxc := 0
	total := 0
	for _, c := range counts {
		if c > maxc {
			maxc = c
		}
		total += c
	}
	if maxc == 0 {
		fmt.Println("No samples to plot")
		return
	}
	scale := float64(width) / float64(maxc)
	for i, c := range counts {
		barLen := int(scale * float64(c))
		bar := strings.Repeat("█", barLen)
		fmt.Printf("%12s |%s %d\n", labels[i], bar, c)
	}
	fmt.Printf("total: %d\n", total)
}
