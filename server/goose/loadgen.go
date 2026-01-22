package goose

import (
	"fmt"
	"math/rand"
	"time"
)

// -------------------- Loadgen implementation --------------------

// Loadgen produces requests into reqCh and consumes replies from repCh.
// - reqCh: where Requests are sent
// - repCh: shared reply channel from workers (Loadgen reads replies here)
// - n: number of requests to generate
// - iatMeanMs: mean inter-arrival time in milliseconds (exponential)
// - waitMeanMs: mean WaitDemand in milliseconds (exponential)
//
// Behavior:
// - For each scheduled arrival (exponential iat), Loadgen attempts a *non-blocking*
//   send of a Request into reqCh. If the send would block, the request is skipped
//   and SendUpcall(..., true) is invoked.
// - Each Request carries ReplyCh set to repCh so workers may reply into the shared reply channel.
// - Loadgen processes replies as they arrive and calls ReceiveUpcall for each.

func Loadgen(reqCh chan<- Request, repCh chan Request, n int, iatMeanMs, waitMeanMs float64) {
	if n <= 0 {
		return
	}
	// ensure stats cleared
	ResetStats()

	// RNG
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	expMs := func(mean float64) time.Duration {
		return time.Duration(r.ExpFloat64() * mean * float64(time.Millisecond))
	}

	sentAttempts := 0
	nextClientID := 0

	// timer scheduling
	var timer *time.Timer
	var timerC <-chan time.Time
	// schedule first arrival
	timer = time.NewTimer(expMs(iatMeanMs))
	timerC = timer.C
	
	startup := time.Now()
	elapsed := time.Since(startup)

	// run until all attempts tried and all outstanding replies handled
	for {
		// termination condition:
		// attempts >= n and no outstanding sendTimes entries (i.e., all replies received for sends)
		statsMu.Lock()
		outstanding := len(sendTimes)
		statsMu.Unlock()
		if sentAttempts >= n && outstanding == 0 {
			// stop timer if active
			if timer != nil {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			break
		}

		select {
		case <-timerC:
			// arrival scheduled
			sentAttempts++
			waitDur := expMs(waitMeanMs)
			req := Request{
				ClientID:   nextClientID,
				ObjectID:   r.Intn(1024),
				WorkDemand: 0,
				WaitDemand: int(waitDur / time.Millisecond),
				ReplyCh:    repCh,
			}
			nextClientID++

			// non-blocking send attempt
			select {
			case reqCh <- req:
				SendUpcall(req, false)
			default:
				// skipped
				SendUpcall(req, true)
			}

			// schedule next if needed
			if sentAttempts < n {
				if timer == nil {
					timer = time.NewTimer(expMs(iatMeanMs))
				} else {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(expMs(iatMeanMs))
				}
				timerC = timer.C
			} else {
				// no more arrivals to schedule
				elapsed = time.Since(startup)

				if timer != nil {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
				}
				timer = nil
				timerC = nil
			}

		case rep, ok := <-repCh:
			if !ok {
				// reply channel closed: no further replies
				// set repCh nil so we don't read again
				repCh = nil
				// continue loop; termination depends on attempts/outstanding
				continue
			}
			// inform stats
			ReceiveUpcall(rep)

		}
	}

	// Loadgen done. leave stats in package globals for caller to inspect/plot.
	seconds := elapsed.Seconds()
	lambda := float64(n) / seconds
	cleartime := time.Since(startup) - elapsed
	fmt.Printf("sent=%d offered load lambda=%.2f/sec, clear time=%dms\n", n, lambda, cleartime.Milliseconds())
}
