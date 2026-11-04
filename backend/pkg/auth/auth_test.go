package auth

import (
	"testing"
	"time"
)

func TestCheckPasswordBackoff(t *testing.T) {
	g := New("right", "/")

	if ok, wait := g.checkPassword("10.0.0.1", "wrong"); ok || wait != 0 {
		t.Fatalf("first failure: ok=%v wait=%v", ok, wait)
	}

	// The failing client must back off, even with the right password...
	if ok, wait := g.checkPassword("10.0.0.1", "right"); ok || wait <= 0 || wait > backoffBase {
		t.Fatalf("during back-off: ok=%v wait=%v", ok, wait)
	}
	// ...while another client logs in immediately.
	if ok, wait := g.checkPassword("10.0.0.2", "right"); !ok || wait != 0 {
		t.Fatalf("other client: ok=%v wait=%v", ok, wait)
	}

	// After the back-off, the right password works and resets the count.
	time.Sleep(backoffBase + 50*time.Millisecond)
	if ok, wait := g.checkPassword("10.0.0.1", "right"); !ok || wait != 0 {
		t.Fatalf("after back-off: ok=%v wait=%v", ok, wait)
	}
	if _, found := g.failures["10.0.0.1"]; found {
		t.Fatal("success should clear the client's failures")
	}
}

func TestBackoffDoublesAndCaps(t *testing.T) {
	g := New("right", "/")
	g.failures["10.0.0.1"] = &failState{count: 2, until: time.Now()} // two earlier failures, back-off just ended
	g.checkPassword("10.0.0.1", "wrong")
	if got := time.Until(g.failures["10.0.0.1"].until); got <= 3*time.Second || got > 4*time.Second {
		t.Fatalf("third failure should wait ~4s, got %v", got)
	}

	g.failures["10.0.0.1"] = &failState{count: 40, until: time.Now()}
	g.checkPassword("10.0.0.1", "wrong")
	if got := time.Until(g.failures["10.0.0.1"].until); got > backoffMax || got < backoffMax-time.Second {
		t.Fatalf("back-off should cap at %v, got %v", backoffMax, got)
	}
}
