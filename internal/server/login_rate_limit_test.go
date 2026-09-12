package server

import (
	"testing"
	"time"
)

func TestLoginRateLimiter(t *testing.T) {
	limiter := newLoginRateLimiter(2, time.Minute)
	now := time.Now()
	if allowed, _ := limiter.allow("192.0.2.1", now); !allowed {
		t.Fatal("first attempt was rejected")
	}
	if allowed, _ := limiter.allow("192.0.2.1", now); !allowed {
		t.Fatal("second attempt was rejected")
	}
	if allowed, retry := limiter.allow("192.0.2.1", now); allowed || retry <= 0 {
		t.Fatalf("third attempt = allowed:%v retry:%s", allowed, retry)
	}
	if allowed, _ := limiter.allow("192.0.2.2", now); !allowed {
		t.Fatal("independent client was rejected")
	}
	if allowed, _ := limiter.allow("192.0.2.1", now.Add(time.Minute)); !allowed {
		t.Fatal("client was not allowed after window reset")
	}
}
