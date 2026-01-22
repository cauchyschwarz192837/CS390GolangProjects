package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	. "courses.cs.duke.edu/go/kvcache"
)

// ----- Example usage (main) -----

func main() {

	// --- NEW: Read Values from Command Line Arguments ---
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run kvrun.go <val1> <val2>")
		os.Exit(1)
	}

	// Parse first argument as integer (val1)
	val1, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("Error parsing val1: %v\n", err)
		os.Exit(1)
	}

	// Parse second argument as integer (val2)
	val2, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Printf("Error parsing val2: %v\n", err)
		os.Exit(1)
	}
	// ----------------------------------------------------

	// Channel to send requests to KV store.
	kvReqCh := make(chan KVRequest)

	var wg sync.WaitGroup

	// Start KV store goroutine.
	wg.Add(1)
	go KVStore(kvReqCh, &wg)

	// Create two client action channels and start clients.
	client1Ch := make(chan ClientAction)
	client2Ch := make(chan ClientAction)

	wg.Add(2)
	go KVClient("client1", client1Ch, kvReqCh, &wg)
	go KVClient("client2", client2Ch, kvReqCh, &wg)

	// Helper: perform a synchronous client action and wait for reply.
	do := func(clientCh chan<- ClientAction, act ClientAction) ClientReply {
		act.Reply = make(chan ClientReply)
		clientCh <- act
		resp := <-act.Reply
		close(act.Reply)
		return resp
	}

	async := func(clientCh chan<- ClientAction, act ClientAction) {
		clientCh <- act
	}

	// Demonstration trace:

	fmt.Println("=== Demo start ===")

	// client1: get "alpha" (not in cache, will cause store to create alpha=0 and return 0)
	resp := do(client1Ch, ClientAction{Type: ClientGet, Key: "alpha"})
	fmt.Printf("[client1] get alpha -> value=%d ok=%v err=%q\n", resp.Value, resp.Ok, resp.Err)

	// client1: put "alpha" -> val1 (read from CLI args)
	resp = do(client1Ch, ClientAction{Type: ClientPut, Key: "alpha", Value: val1})
	fmt.Printf("[client1] put alpha=%d -> ok=%v err=%q\n", val1, resp.Ok, resp.Err)

	// client2: get "alpha" (not in cache, should read and return val1 )
	resp = do(client2Ch, ClientAction{Type: ClientGet, Key: "alpha"})
	fmt.Printf("[client2] get alpha -> value=%d ok=%v err=%q\n", resp.Value, resp.Ok, resp.Err)

	// client1: get "alpha" (owned by client2: use async)
	pending := ClientAction{Type: ClientGet, Key: "alpha", Reply: make(chan ClientReply, 2)}
	async(client1Ch, pending)
	fmt.Printf("[client1] get alpha (pending)\n")

	time.Sleep(10 * time.Millisecond)

	// client2: get "alpha" (cache hit, returns val1 )
	resp = do(client2Ch, ClientAction{Type: ClientGet, Key: "alpha"})
	fmt.Printf("[client2] get alpha -> value=%d ok=%v err=%q\n", resp.Value, resp.Ok, resp.Err)

	// client2: put "alpha" -> val2 (read from CLI args)
	resp = do(client2Ch, ClientAction{Type: ClientPut, Key: "alpha", Value: val2})
	fmt.Printf("[client2] put alpha=%d -> ok=%v err=%q\n", val2, resp.Ok, resp.Err)

	// harvest get response from client1; gets val1 if inconsistent, val2 if consistent. Client1 owns alpha now.
	resp = <-pending.Reply
	close(pending.Reply)
	fmt.Printf("[client1] pending get alpha reply -> value=%d ok=%v err=%q\n", resp.Value, resp.Ok, resp.Err)

	// Wait a short moment to let goroutines finish their prints (not strictly needed).
	time.Sleep(100 * time.Millisecond)

	// Close client channels to stop client goroutines.
	close(client1Ch)
	close(client2Ch)

	// Close the KV request channel to stop the KV store goroutine.
	close(kvReqCh)

	// Wait for goroutines to finish.
	wg.Wait()

	fmt.Println("=== Demo end ===")
}
