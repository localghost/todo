package auth

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// At most hashSlotCount argon2 hashes run at the same time, so parallel
// logins cannot use unbounded memory (64 MiB each).
func TestHashSlotsLimitParallelHashes(t *testing.T) {
	var running, peak atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 3*hashSlotCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			withHashSlot(context.Background(), func() {
				n := running.Add(1)
				for {
					p := peak.Load()
					if n <= p || peak.CompareAndSwap(p, n) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				running.Add(-1)
			})
		}()
	}
	wg.Wait()
	if p := peak.Load(); p > hashSlotCount || p == 0 {
		t.Fatalf("peak parallel hashes = %d, want 1..%d", p, hashSlotCount)
	}
}

// The parallel password checks must fit in a 256 MB Fly machine together with
// the app and the VM's Linux (docs/superpowers/specs/2026-10-02-hosting-design.md).
func TestParallelHashesFitSmallMachine(t *testing.T) {
	if mib := hashSlotCount * argonMemory / 1024; mib > 128 {
		t.Fatalf("parallel password checks can use %d MiB, want at most 128", mib)
	}
}

// fillHashSlots takes all hash slots, as if hashes were running, and returns
// a function that frees them.
func fillHashSlots() (free func()) {
	for i := 0; i < hashSlotCount; i++ {
		hashSlots <- struct{}{}
	}
	return func() {
		for i := 0; i < hashSlotCount; i++ {
			<-hashSlots
		}
	}
}

// waitForWaiters waits until n requests wait for a hash slot.
func waitForWaiters(t *testing.T, n int32) {
	t.Helper()
	for end := time.Now().Add(2 * time.Second); hashWaiters.Load() != n; time.Sleep(time.Millisecond) {
		if time.Now().After(end) {
			t.Fatalf("waiters = %d, want %d", hashWaiters.Load(), n)
		}
	}
}

// When maxHashWaiters requests wait for a slot, the next one fails at once
// with ErrBusy; it does not wait until the request times out (code review 2).
func TestHashQueueIsBounded(t *testing.T) {
	free := fillHashSlots()
	var wg sync.WaitGroup
	var ran atomic.Int32
	for i := 0; i < maxHashWaiters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := withHashSlot(context.Background(), func() { ran.Add(1) }); err != nil {
				t.Errorf("waiter: %v", err)
			}
		}()
	}
	waitForWaiters(t, maxHashWaiters)
	start := time.Now()
	if err := withHashSlot(context.Background(), func() { t.Error("the refused work ran") }); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Fatalf("ErrBusy took %v, want at once", d)
	}
	free()
	wg.Wait()
	if ran.Load() != maxHashWaiters || hashWaiters.Load() != 0 || len(hashSlots) != 0 {
		t.Fatalf("ran %d, waiters %d, slots taken %d", ran.Load(), hashWaiters.Load(), len(hashSlots))
	}
}

// A request that ends while it waits for a slot stops waiting and takes no slot.
func TestHashWaitStopsWithTheRequest(t *testing.T) {
	free := fillHashSlots()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- withHashSlot(ctx, func() { t.Error("the cancelled work ran") }) }()
	waitForWaiters(t, 1)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if hashWaiters.Load() != 0 {
		t.Fatalf("waiters = %d, want 0", hashWaiters.Load())
	}
	free()
	if len(hashSlots) != 0 {
		t.Fatalf("slots taken = %d, want 0", len(hashSlots))
	}
}
