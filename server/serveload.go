package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	. "courses.cs.duke.edu/go/goose"
)

func main() {
	const N = 1000 // number of requests

	// --- NEW: Read Parameters from Command Line ---
	if len(os.Args) < 4 {
		fmt.Printf("Usage: %s <iatMean> <demandMean> <maxConcurrent>\n", os.Args[0])
		os.Exit(1)
	}

	iatMean, err := strconv.ParseFloat(os.Args[1], 64)
	if err != nil {
		log.Fatalf("Invalid iatMean: %v", err)
	}

	demandMean, err := strconv.ParseFloat(os.Args[2], 64)
	if err != nil {
		log.Fatalf("Invalid demandMean: %v", err)
	}

	maxConcurrent, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Fatalf("Invalid maxConcurrent: %v", err)
	}
	// ----------------------------------------------

	reqCh := make(chan Request, 16)
	repCh := make(chan Request, 16)

	// Start handler
	go ReqHandler(reqCh, maxConcurrent)

	startup := time.Now()

	// Let's go goose!
	Loadgen(reqCh, repCh, N, iatMean, demandMean)

	elapsed := time.Since(startup)

	//--------------------------------------------------------------------------------------

	// After Loadgen returns, get stats and histogram
	attempts, sent, skipped, recv, mean := GetStats()
	if attempts != sent+skipped {
		fmt.Printf("Dropped %d attempts somehow: should not happen.\n", attempts-(sent+skipped))
	}
	if recv != sent {
		fmt.Printf("Reported %d sends without replies: should not happen.\n", sent-recv)
	}
	seconds := elapsed.Seconds()
	throughput := float64(recv) / seconds
	fmt.Printf("sent=%d skipped=%d throughput=%.0f/sec meanRT=%.3fms\n",
		sent, skipped, throughput, mean)

	// build histogram: 10 bins up to 100ms, last bin is >=100ms
	counts, labels := HistogramLinear(10, 100.0)
	PrintHistogramASCII(counts, labels, 60)

	close(reqCh) // let handler finish (it will close repCh)
}
