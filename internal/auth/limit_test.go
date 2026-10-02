package auth

import (
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
			withHashSlot(func() {
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
