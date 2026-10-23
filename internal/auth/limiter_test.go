package auth

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestLoginLimiterBlocksByAccountAndIP(t *testing.T) {
	now := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	limiter := NewLoginLimiter()
	limiter.SetClock(func() time.Time { return now })

	for i := 0; i < 6; i++ {
		if ok, _ := limiter.Allow("10.0.0.8", "Alice"); !ok {
			t.Fatalf("attempt %d should be allowed before the account limit", i+1)
		}
		limiter.Fail("10.0.0.8", "Alice")
	}
	ok, wait := limiter.Allow("10.0.0.8", "alice")
	if ok || wait <= 0 {
		t.Fatalf("account limit should block case-insensitive login, got ok=%v wait=%v", ok, wait)
	}
	if ok, _ := limiter.Allow("10.0.0.8", "Bob"); !ok {
		t.Fatal("another account from the same IP must still be allowed under the IP ceiling")
	}

	for i := 0; i < 6; i++ {
		limiter.Fail("10.0.0.8", fmt.Sprintf("student-%d", i))
	}
	if ok, wait := limiter.Allow("10.0.0.8", "fresh-account"); ok || wait <= 0 {
		t.Fatalf("per-IP ceiling should block after 12 total failures, got ok=%v wait=%v", ok, wait)
	}
	if ok, _ := limiter.Allow("10.0.0.9", "fresh-account"); !ok {
		t.Fatal("a different IP bucket must remain independent")
	}
}

func TestLoginLimiterResetsAccountButKeepsIPCeiling(t *testing.T) {
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	limiter := NewLoginLimiter()
	limiter.SetClock(func() time.Time { return now })

	for i := 0; i < 5; i++ {
		limiter.Fail("192.168.1.20", "Alice")
	}
	limiter.Reset("alice")
	if ok, _ := limiter.Allow("192.168.1.20", "ALICE"); !ok {
		t.Fatal("successful login must clear the account bucket")
	}

	for i := 0; i < 7; i++ {
		limiter.Fail("192.168.1.20", fmt.Sprintf("other-%d", i))
	}
	if ok, _ := limiter.Allow("192.168.1.20", "Alice"); ok {
		t.Fatal("Reset must not erase the shared per-IP brute-force budget")
	}
}

func TestLoginLimiterWindowExpiresAndSweepDropsStaleBuckets(t *testing.T) {
	now := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	limiter := NewLoginLimiter()
	limiter.SetClock(func() time.Time { return now })

	for i := 0; i < 6; i++ {
		limiter.Fail("172.16.0.5", "Alice")
	}
	if ok, _ := limiter.Allow("172.16.0.5", "Alice"); ok {
		t.Fatal("expected account bucket to be exhausted")
	}

	now = now.Add(5*time.Minute + time.Second)
	if ok, _ := limiter.Allow("172.16.0.5", "Alice"); !ok {
		t.Fatal("failures older than the sliding window must stop counting")
	}

	limiter.Fail("172.16.0.99", "stale-user")
	now = now.Add(2 * 5 * time.Minute)
	limiter.Sweep()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.entries) != 0 {
		t.Fatalf("Sweep should remove stale buckets, %d remain", len(limiter.entries))
	}
}

func TestLoginLimiterConcurrentAccessIsRaceFree(t *testing.T) {
	limiter := NewLoginLimiter()
	base := time.Date(2026, 10, 21, 11, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	now := base
	limiter.SetClock(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	})

	var wg sync.WaitGroup
	for worker := 0; worker < 24; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				ip := fmt.Sprintf("10.1.%d.%d", worker%3, i%7)
				login := fmt.Sprintf("user-%d", i%9)
				_, _ = limiter.Allow(ip, login)
				limiter.Fail(ip, login)
				if i%5 == 0 {
					limiter.Reset(login)
				}
				if i%11 == 0 {
					limiter.Sweep()
				}
			}
		}(worker)
	}
	wg.Wait()
}

func TestNilLoginLimiterIsSafe(t *testing.T) {
	var limiter *LoginLimiter
	if ok, wait := limiter.Allow("", ""); !ok || wait != 0 {
		t.Fatalf("nil limiter should fail open, got ok=%v wait=%v", ok, wait)
	}
	limiter.Fail("", "")
	limiter.Reset("")
	limiter.Sweep()
	limiter.SetClock(nil)
}
