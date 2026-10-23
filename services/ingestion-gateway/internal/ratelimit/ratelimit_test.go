package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := New(10, 3)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("burst request %d denied", i)
		}
	}
	if l.Allow("a") {
		t.Fatal("request beyond burst allowed")
	}
	if !l.Allow("b") {
		t.Fatal("keys must be independent")
	}
	now = now.Add(100 * time.Millisecond) // refills exactly one token at 10/s
	if !l.Allow("a") {
		t.Fatal("token should have refilled")
	}
	if l.Allow("a") {
		t.Fatal("only one token should have refilled")
	}
}

func TestDisabled(t *testing.T) {
	l := New(0, 0)
	for i := 0; i < 1000; i++ {
		if !l.Allow("a") {
			t.Fatal("disabled limiter denied a request")
		}
	}
}
