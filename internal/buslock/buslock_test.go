package buslock

import "testing"

func TestLock_AcquireRelease(t *testing.T) {
	l := New()
	ok, owner := l.TryAcquire("pairing")
	if !ok {
		t.Fatalf("first acquire should succeed")
	}
	if owner != "" {
		t.Fatalf("owner on success should be empty, got %q", owner)
	}

	ok2, cur := l.TryAcquire("gridprofile-broadcast")
	if ok2 {
		t.Fatalf("second acquire should fail while held")
	}
	if cur != "pairing" {
		t.Fatalf("current owner = %q want pairing", cur)
	}

	if held, who := l.Held(); !held || who != "pairing" {
		t.Fatalf("Held = (%v,%q) want (true,pairing)", held, who)
	}

	l.Release()
	if held, _ := l.Held(); held {
		t.Fatalf("lock still held after release")
	}

	// Now the other owner can take it.
	ok3, _ := l.TryAcquire("gridprofile-broadcast")
	if !ok3 {
		t.Fatalf("acquire after release should succeed")
	}
	l.Release()
}

func TestLock_DoubleReleaseSafe(t *testing.T) {
	l := New()
	l.Release() // no-op, must not panic
	ok, _ := l.TryAcquire("x")
	if !ok {
		t.Fatal("acquire after spurious release should succeed")
	}
	l.Release()
	l.Release() // double release, must not panic
}
