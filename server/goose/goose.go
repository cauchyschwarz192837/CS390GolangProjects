//go:build !dev

package goose

import (
	//	"sync"
	"time"
)

type Request struct {
	ClientID   int
	ObjectID   int
	WorkDemand int // milliseconds (CPU work)
	WaitDemand int // milliseconds (sleep)
	ReplyCh    chan<- Request
}

func ReqHandler(reqCh <-chan Request, maxConcurrent int) {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	for req := range reqCh {
		serve(req)
	}
}

// Serve one request.  Sleep or burnCPU as requested.
func serve(r Request) {
	if r.WorkDemand > 0 {
		burnCPU(r.WorkDemand)
	}
	if r.WaitDemand > 0 {
		time.Sleep(time.Duration(r.WaitDemand) * time.Millisecond)
	}

	if r.ReplyCh != nil {
		r.ReplyCh <- r
	}
}

func burnCPU(ms int) {
	deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)

	var x uint64 = 1

	for time.Now().Before(deadline) {
		x = x*1664525 + 1013904223
	}

	// Use it or lose it: touch x so compiler cannot optimize it all away
	_ = x
}
