package auth

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const maxLimiterEntries = 100_000

// LoginLimiter is a small in-process sliding-window brute-force guard. It is
// intentionally keyed by both client IP and login identifier so one noisy
// classroom NAT cannot lock every account, while an attacker rotating accounts
// from one machine still hits the per-IP ceiling.
type LoginLimiter struct {
	mu        sync.Mutex
	now       func() time.Time
	window    time.Duration
	ipLimit   int
	keyLimit  int
	entries   map[string]*loginWindow
	nextSweep time.Time
}

type loginWindow struct {
	failures []time.Time
}

// NewLoginLimiter builds a limiter with sane classroom defaults:
// 12 failures per IP per 5 minutes, 6 failures per account per 5 minutes.
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		now:      time.Now,
		window:   5 * time.Minute,
		ipLimit:  12,
		keyLimit: 6,
		entries:  make(map[string]*loginWindow),
	}
}

// SetClock is used by tests and lets the transport share its process clock.
func (l *LoginLimiter) SetClock(now func() time.Time) {
	if now == nil {
		return
	}
	l.mu.Lock()
	l.now = now
	l.nextSweep = time.Time{}
	l.mu.Unlock()
}

// Allow reports whether a login attempt may proceed. It does not count the
// attempt; call Fail after an invalid credential result. This keeps successful
// logins cheap and ensures typos on a valid account are still rate limited.
func (l *LoginLimiter) Allow(ip, login string) (bool, time.Duration) {
	if l == nil {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	ipKey := "ip:" + strings.TrimSpace(ip)
	userKey := "login:" + strings.ToLower(strings.TrimSpace(login))
	ipWait := l.retryAfterLocked(ipKey, l.ipLimit, now)
	userWait := l.retryAfterLocked(userKey, l.keyLimit, now)
	wait := ipWait
	if userWait > wait {
		wait = userWait
	}
	return wait == 0, wait
}

func (l *LoginLimiter) retryAfterLocked(key string, limit int, now time.Time) time.Duration {
	entry := l.entries[key]
	if entry == nil || limit <= 0 {
		return 0
	}
	cutoff := now.Add(-l.window)
	kept := entry.failures[:0]
	for _, at := range entry.failures {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	entry.failures = kept
	if len(kept) < limit {
		return 0
	}
	// The oldest still-relevant failure determines when a slot frees up.
	return kept[0].Add(l.window).Sub(now)
}

// Fail records one invalid credential attempt.
func (l *LoginLimiter) Fail(ip, login string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if l.nextSweep.IsZero() || !now.Before(l.nextSweep) {
		l.sweepLocked(now)
		l.nextSweep = now.Add(l.window)
	}
	for _, key := range []string{
		"ip:" + strings.TrimSpace(ip),
		"login:" + strings.ToLower(strings.TrimSpace(login)),
	} {
		entry := l.entries[key]
		if entry == nil {
			entry = &loginWindow{}
			l.entries[key] = entry
		}
		entry.failures = append(entry.failures, now)
	}
	if len(l.entries) > maxLimiterEntries {
		l.trimLocked(now)
	}
}

// Reset clears the account bucket after a successful login. The IP bucket is
// deliberately retained so a valid account cannot be used to bypass the
// broader credential-stuffing ceiling.
func (l *LoginLimiter) Reset(login string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, "login:"+strings.ToLower(strings.TrimSpace(login)))
}

// Sweep drops stale buckets so a long-running server does not grow memory with
// every username typo.
func (l *LoginLimiter) Sweep() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweepLocked(now)
	l.nextSweep = now.Add(l.window)
}

func (l *LoginLimiter) sweepLocked(now time.Time) {
	cutoff := now.Add(-2 * l.window)
	for key, entry := range l.entries {
		if len(entry.failures) == 0 || !entry.failures[len(entry.failures)-1].After(cutoff) {
			delete(l.entries, key)
		}
	}
}

// trimLocked is the memory-hard cap behind the periodic sweep. A botnet can
// still create more than maxLimiterEntries keys inside one window, so evict the
// least-recently-used buckets rather than allowing process memory to grow
// without bound.
func (l *LoginLimiter) trimLocked(now time.Time) {
	type candidate struct {
		key string
		at  time.Time
	}
	items := make([]candidate, 0, len(l.entries))
	for key, entry := range l.entries {
		last := time.Time{}
		if len(entry.failures) > 0 {
			last = entry.failures[len(entry.failures)-1]
		}
		items = append(items, candidate{key: key, at: last})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].at.Equal(items[j].at) {
			return items[i].key < items[j].key
		}
		return items[i].at.Before(items[j].at)
	})
	remove := len(items) - maxLimiterEntries
	for i := 0; i < remove; i++ {
		delete(l.entries, items[i].key)
	}
	// Keep clock-derived state bounded even if the clock moved backwards.
	_ = now
}
