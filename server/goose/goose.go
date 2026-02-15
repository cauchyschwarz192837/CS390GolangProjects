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

type Permission struct{}

// OK!
func ReqHandler(reqCh <-chan Request, maxConcurrent int) {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	// CHANNEL MUST STORE PERMITS, NOT REQUESTS!
	// use a channel as a counting semaphore
	permissions := make(chan Permission, maxConcurrent)

	for req := range reqCh {
		perm := Permission{}
		permissions <- perm

		go serve(req, permissions)
	}
}

func byebye(permissions <-chan Permission) {
	<-permissions
}

// When a thread blocks:
// OS saves its registers
// Marks it BLOCKED
// Picks another READY thread
// Loads its registers
// CPU continues immediately
// data := <-ch    // blocks goroutine

// Serve one request.  Sleep or burnCPU as requested.
// fire goroutine for each request,
func serve(r Request, permissions <-chan Permission) {

	// Deferred calls run in LIFO order (stack behavior)
	defer byebye(permissions)

	if r.WorkDemand > 0 {
		burnCPU(r.WorkDemand) // spins, prevents other work in the same goroutine
	}
	if r.WaitDemand > 0 {
		time.Sleep(time.Duration(r.WaitDemand) * time.Millisecond) // blocking operation
	}

	if r.ReplyCh != nil {
		r.ReplyCh <- r
	}
}

// OK!
func burnCPU(ms int) {
	deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)

	var x uint64 = 1

	for time.Now().Before(deadline) {
		x = x*1664525 + 1013904223
	}

	// Use it or lose it: touch x so compiler cannot optimize it all away
	_ = x
}
