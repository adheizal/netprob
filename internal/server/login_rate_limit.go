package server

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type loginAttemptWindow struct {
	started time.Time
	count   int
}

type loginRateLimiter struct {
	mu          sync.Mutex
	maxAttempts int
	window      time.Duration
	attempts    map[string]loginAttemptWindow
	lastCleanup time.Time
}

func newLoginRateLimiter(maxAttempts int, window time.Duration) *loginRateLimiter {
	return &loginRateLimiter{
		maxAttempts: maxAttempts,
		window:      window,
		attempts:    make(map[string]loginAttemptWindow),
	}
}

func (l *loginRateLimiter) allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) >= l.window {
		for candidate, state := range l.attempts {
			if now.Sub(state.started) >= l.window {
				delete(l.attempts, candidate)
			}
		}
		l.lastCleanup = now
	}

	state, exists := l.attempts[key]
	if !exists || now.Sub(state.started) >= l.window {
		l.attempts[key] = loginAttemptWindow{started: now, count: 1}
		return true, 0
	}
	if state.count >= l.maxAttempts {
		return false, l.window - now.Sub(state.started)
	}
	state.count++
	l.attempts[key] = state
	return true, 0
}

func loginClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
