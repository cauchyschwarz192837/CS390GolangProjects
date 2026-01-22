
# Lab 1: Caching with Ownership

We call this lab **duality** because it illustrates the duality of locking and channel-based synchronization.   You are asked to implement simple locks using message passing.  We apply the idea in a design pattern that is common in networked services: a *key-value store*. 

A key-value store is a service that maps keys to values.  Internally, they are maps or hash tables: what is important about KV stores is that they support associative lookup (random access) to a sparse set of keys.  The keys and values could be of any type.  To keep it simple, our keys are strings and the values are ints.   The clients of a KV store (or any storage system) may keep a copy of a key-value mapping (a KV pair) in local memory after lookup, to reduce the need to contact the KV store to look up the same keys if they are needed later.   This is what we mean by KV cache.

This use of *KV cache* is not to be confused with the use of the same term in AI/LLMs.    Science, like programming, involves naming things.   But there are a lot of things and only so many names, so we have to reuse the names.   Terms often have multiple meanings (*overloaded*).   A KV cache in LLMs is a data structure that associates pairs of vectors of numbers, but they are built for sequential access over a sliding time window, and not associative access.

## KVStore and KVClient

This simple package emulates a key-value store (KVStore) and a group of clients (KVClient) that maintain a cache over the KVStore.

KVStore runs as a single server goroutine.   KVStore receives and handles requests from client goroutines over a request channel until the channel is closed (e.g., by main). 

A KVClient is a goroutine that sends requests to KVStore over KVStore's request channel.   To send a request, the client sends a KVRequest struct into the request channel.    Each request specifies a channel for the KVStore to reply to the client.   The client waits for the reply on the reply channel.   Thus clients have at most one outstanding request at a time.

KVStore supports *read* and *write* operations.   For read, if the specified key is not in the map, KVStore creates an entry with value 0.   A write request fails if the key is not in the map, else it updates the value for the key to a value specified in the request.

KVClients also receive requests over a per-client channel.   The request types are *get* and *put* on a specified key. The clients keep a local cache of key-value pairs.   A *get* request may return a value from the cache, if the requested key is present, else it sends a request to read the pair from KVStore, caches it, and replies with the value.   A *put* request writes a new value for the key to KVStore, and removes the key from the local cache.   This use of get/put is an arbitrary choice for the client application/API, but it is common in systems and it serves our purposes.  Get means "I will be using this key and possibly modifying its value", and put means "I am done with this key for now".

## The Problem: Consistent Caching

Your assignment is to modify the code for KVStore to ensure that the client caches are *consistent* in the following sense: every *get* returns the value of the most recent write to the requested key, or zero if it is the first access to the key.   More precisely, this property is a strong form of *cache coherence*.

## Concepts of a Solution

The approach we use resembles the locking scheme in distributed file systems like the Network File System (NFS), parallel file systems in supercomputers, and coordination services like Chubby, Zookeeper, or etcd.   It is also similar to an ownership scheme, sometimes called MESI, that keeps processor caches coherent in multi-core architectures.

A client with a cached key *K* must *own* K.   Ownership permits the client to access its cached copy.   Real systems distinguish ownership in read mode (Shared) vs. ownership in write mode (Exclusive): at most one client may own any K in write mode, but if no client does, then many clients may own K in read mode.   Our variant is simplistic: it has only write mode, which also allows reading.

In real systems (e.g., NFS or Chubby), the server may demand that a client relinquish ownership at any time, in order to grant the ownership to another client.   Our variant is simplistic: clients relinquish ownership on *put*, which may also write a new value for K.

Our variant is simplistic in yet another way: it does not handle client failures.   Real systems use timed grants called *leases* to handle client failures.   We discuss that later in the semester.

## What to Do

Modify KVStore to track ownership of keys.   If a client W requests a read on a key K owned by another client, then W must wait until the owner of K releases ownership of K.   Track waiters and reply to W as soon as KVStore can grant ownership of K to W.

You should modify only KVStore.   You do not need to modify KVClient or any of the struct type definitions.

To summarize:
- KVStore permits at most one client to have ownership of a given key K at any given time.
- KVStore implicitly grants ownership of a key K to the client on any reply to a read on K.
- A client implicitly releases ownership of K on a write to K (when the client receives a put request on K).

To keep your code simple, you may assume that at most one client is waiting for ownership of any given key at any given time.

More generally, our simplistic approach relies on the main program (e.g., the autograder) to generate a well-behaved workload.   At most two clients contend for a key at a time, and every get has a matching put.

## Dealing with Deadlock

Main submits get/put requests to clients in a sequence.   If a client fails to reply when main expects it to reply, then the emulation will *deadlock* because main waits for the reply and cannot submit its next request.   That may occur if the client should be able to acquire ownership of the key but is erroneously forced to wait, e.g., because your code is broken.    Deadlock is a likely outcome of concurrency bugs in Go programs.   When it occurs, the runtime system will output the line numbers where all the threads (main and goroutines) are blocked.   You should be able to figure out what went wrong from that information.


## Testing

To run tests with your code, use the following go command to execute the kvcache server/client program.
```
go run kvrun.go <client 1 value> <client 2 value>
```

For example, try run 
```
go run kvrun.go 42 7
```

and you will see the following output
```
=== Demo start ===
[client1] get alpha -> value=0 ok=true err=""
[client1] put alpha=42 -> ok=true err=""
[client2] get alpha -> value=42 ok=true err=""
[client1] get alpha (pending)
[client2] get alpha -> value=42 ok=true err=""
[client2] put alpha=7 -> ok=true err=""
[client1] pending get alpha reply -> value=42 ok=true err=""
=== Demo end ===
```

Notice that client 1 is getting value 42 after client 2 put a 7 into the kvcache. After you correctly implement a new KVStore function with ownership control, you should see client 1 getting value 7 for alpha.

There is also a python script to run autograder for the lab. Run the following command to use it. 
```
python3 run_tests.py
```

The scripts and test cases are identical to the ones on Gradescope. There are no hidden tests.

## Submission

Summit your code to Coursework(Gitlab) with Git. After that go to gradescope and click submit through gitlab.

```
git add .
git commit -am "<your commit message>"
git push
```