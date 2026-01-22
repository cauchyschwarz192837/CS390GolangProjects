//go:build !dev

package kvcache

import (
	"fmt"
	"sync"
)

// ----- KV store request/response types -----

type KVOp string

const (
	KVRead  KVOp = "read"
	KVWrite KVOp = "write"
)

// KVRequest is a request to KVStore.
type KVRequest struct {
	Op    KVOp         // has an operation, which must be a KVOp (KVRead or KVWrite)
	Key   string       // refers to a key in the key-value store
	Value int          // only used for write
	Reply chan KVReply // channel to send the result back
}

// KVReply is the store's reply.
type KVReply struct {
	Value int  // value for reads; for writes, returned value after update (if Ok)
	Ok    bool // true on success; false on failure (e.g., write to missing key)
}

// ----- Client action/request types -----

type ClientActionType string

const (
	ClientGet ClientActionType = "get"
	ClientPut ClientActionType = "put"
)

// ClientAction is received by the client goroutine from its callers.
type ClientAction struct {
	Type  ClientActionType
	Key   string
	Value int // used for put
	Reply chan ClientReply
}

// ClientReply is the client's reply to the caller that initiated a get/put.
type ClientReply struct {
	Value int
	Hit   bool
	Ok    bool
	Err   string // optional human-friendly error
}

// ----- Key-Value store goroutine -----

// KVStore runs as a goroutine and services KVRequest messages until reqCh is closed.
func KVStore(reqCh <-chan KVRequest, wg *sync.WaitGroup) {
	defer wg.Done()
	store := make(map[string]int)
	// keyholder_store := make(map[string]chan KVReply) // the string is the relevant key, the channel KVReply from the KVRequest sent
	isKeyOwned_store := make(map[string]bool)
	waitingclients_store := make(map[string]KVRequest)

	// KVClient has no client ID

	for req := range reqCh { // blocks until a request arrives // THIS IS KVREQUESTS!
		switch req.Op {
		// Grant ownership on reading on key K
		case KVRead:
			// If key missing, create with 0.
			if !isKeyOwned_store[req.Key] {
				isKeyOwned_store[req.Key] = true

				val, ok := store[req.Key] // retrieve
				if !ok {
					store[req.Key] = 0
					val = 0
				}
				req.Reply <- KVReply{Value: val, Ok: true}
			} else {
				waitingclients_store[req.Key] = req
			}
			// If req.Reply is an unbuffered channel, this send will block
			// until the client receives from it. store will pause here
			// until someone reads the reply

		// Relinquish ownership on writing on key K
		case KVWrite:

			// Fail if key not in map.
			if _, ok := store[req.Key]; !ok {
				req.Reply <- KVReply{Value: 0, Ok: false}
			} else {
				store[req.Key] = req.Value
				req.Reply <- KVReply{Value: req.Value, Ok: true}

				isKeyOwned_store[req.Key] = false

				if waiting_guy, ok := waitingclients_store[req.Key]; ok {
					isKeyOwned_store[waiting_guy.Key] = true
					val, ok := store[waiting_guy.Key] // retrieve
					if !ok {
						store[waiting_guy.Key] = 0
						val = 0
					}
					waiting_guy.Reply <- KVReply{Value: val, Ok: true}
				}
				delete(waitingclients_store, req.Key)

			}

		default:
			// Unknown operation: respond with failure.
			fmt.Println("Invalid operation to kvstore")
			req.Reply <- KVReply{Value: 0, Ok: false}
		}
	}
}

// ----- Client goroutine -----

// KVClient runs as a client goroutine that listens on actionsCh for get/put requests.
// It keeps a local cache (map[string]int). It talks to the KV store via kvReqCh.
func KVClient(name string, actionsCh <-chan ClientAction, kvReqCh chan<- KVRequest, wg *sync.WaitGroup) {
	defer wg.Done()
	cache := make(map[string]int)

	for act := range actionsCh {
		switch act.Type {
		case ClientGet:
			// If in cache, reply immediately.
			if v, ok := cache[act.Key]; ok {
				act.Reply <- ClientReply{Value: v, Hit: true, Ok: true}
				continue
			}
			// Not in cache: send a read to the KV store.
			kvReplyCh := make(chan KVReply) // new one
			kvReq := KVRequest{             // new one
				Op:    KVRead,
				Key:   act.Key,
				Reply: kvReplyCh, // new channel goes in here
			}
			kvReqCh <- kvReq      // send to store
			kvResp := <-kvReplyCh // receive from store
			close(kvReplyCh)

			if kvResp.Ok {
				// populate cache and reply with value
				cache[act.Key] = kvResp.Value
				act.Reply <- ClientReply{Value: kvResp.Value, Hit: false, Ok: true}
			} else {
				act.Reply <- ClientReply{Ok: false, Err: "kv read failed"}
			}

		case ClientPut:
			// Put only allowed if key present in local cache.
			if _, ok := cache[act.Key]; !ok {
				act.Reply <- ClientReply{Ok: false, Err: "key not in local cache"}
				continue
			}

			cache[act.Key] = act.Value

			// Send write to KV store.
			kvReplyCh := make(chan KVReply)
			kvReq := KVRequest{
				Op:    KVWrite,
				Key:   act.Key,
				Value: act.Value,
				Reply: kvReplyCh,
			}
			kvReqCh <- kvReq
			kvResp := <-kvReplyCh
			close(kvReplyCh)

			if kvResp.Ok {
				// Remove from cache after successful put, and reply success.
				delete(cache, act.Key)
				act.Reply <- ClientReply{Hit: true, Ok: true}
			} else {
				act.Reply <- ClientReply{Ok: false, Err: "kv write failed (key missing in store)"}
			}

		default:
			act.Reply <- ClientReply{Ok: false, Err: "unknown action"}
		}
	}
}
