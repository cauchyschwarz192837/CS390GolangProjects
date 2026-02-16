# Golang Projects for CS390: Distributed Systems (Spring 2026)

---

## Lab 1
Emulates a key-value store (KVStore) and a group of clients (KVClient) that maintain a cache over the KVStore, and ensures cache coherence.

## Lab 2
Using threads to improve server performance.

## Lab 3
Different approach to server concurrency often used in scalable services with a service set of multiple back-end servers. we create a WorkCrew of maxConcurrent server goroutines to stand in for the service set, and create ReqHandler (a request handler) that directs each request to the server (worker) with the selected index in the WorkCrew.
