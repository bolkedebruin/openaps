package settings

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// stubRunCmd swaps runCmd for the duration of the test, returning a
// pointer to the captured invocations and a restore func.
type capturedInvocation struct {
	Name string
	Args []string
}

func stubRunCmd(t *testing.T, fn func(call int, name string, args []string) ([]byte, error)) *[]capturedInvocation {
	t.Helper()
	orig := runCmd
	calls := 0
	captured := []capturedInvocation{}
	runCmd = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		captured = append(captured, capturedInvocation{Name: name, Args: append([]string(nil), args...)})
		idx := calls
		calls++
		return fn(idx, name, args)
	}
	t.Cleanup(func() { runCmd = orig })
	return &captured
}

func stubReadMAC(t *testing.T, fn func() string) {
	t.Helper()
	orig := readMACFile
	readMACFile = fn
	t.Cleanup(func() { readMACFile = orig })
}

// shortenVerify cuts the verify delay/poll/timeout for tests.
func shortenVerify(t *testing.T) {
	t.Helper()
	origDelay, origTO, origPoll := applyMACVerifyDelay, applyMACVerifyTimeout, applyMACVerifyPoll
	// Cannot reassign consts; mirror the strategy by overriding the
	// package vars instead. Re-declared as vars at package level for
	// test friendliness if needed. For now we don't touch them — the
	// production 200ms+2s window is acceptable in tests.
	_ = origDelay
	_ = origTO
	_ = origPoll
}

func TestApplyMAC_HappyPath(t *testing.T) {
	const wantMAC = "aa:bb:cc:dd:ee:ff"
	got := stubRunCmd(t, func(call int, name string, args []string) ([]byte, error) {
		return nil, nil
	})
	stubReadMAC(t, func() string { return wantMAC })

	if err := ApplyMAC(context.Background(), wantMAC); err != nil {
		t.Fatalf("ApplyMAC: %v", err)
	}

	wantSeq := []capturedInvocation{
		{Name: "/bin/ip", Args: []string{"link", "set", "dev", "eth0", "down"}},
		{Name: "/bin/ip", Args: []string{"link", "set", "dev", "eth0", "address", wantMAC}},
		{Name: "/bin/ip", Args: []string{"link", "set", "dev", "eth0", "up"}},
	}
	if !reflect.DeepEqual(*got, wantSeq) {
		t.Fatalf("argv sequence mismatch:\n got: %+v\nwant: %+v", *got, wantSeq)
	}
}

func TestApplyMAC_RejectsInvalidMAC(t *testing.T) {
	cases := []string{
		"; rm -rf /",
		"aa:bb:cc:dd:ee",     // 5 octets
		"aabbccddeeff",       // bare hex
		"",                   // empty
		"aa:bb:cc:dd:ee:zz",  // non-hex
		"aa:bb:cc:dd:ee:ff0", // octet too long
	}
	for _, mac := range cases {
		t.Run(mac, func(t *testing.T) {
			got := stubRunCmd(t, func(call int, name string, args []string) ([]byte, error) {
				t.Fatalf("runCmd must NOT be invoked for invalid mac %q", mac)
				return nil, nil
			})
			stubReadMAC(t, func() string {
				t.Fatalf("readMACFile must NOT be invoked for invalid mac %q", mac)
				return ""
			})
			err := ApplyMAC(context.Background(), mac)
			if err == nil {
				t.Fatalf("ApplyMAC(%q): expected error, got nil", mac)
			}
			if len(*got) != 0 {
				t.Fatalf("ApplyMAC(%q): runCmd called %d times, want 0", mac, len(*got))
			}
		})
	}
}

func TestApplyMAC_StepDownFails(t *testing.T) {
	const wantMAC = "aa:bb:cc:dd:ee:ff"
	boom := errors.New("ip: down failed")
	got := stubRunCmd(t, func(call int, name string, args []string) ([]byte, error) {
		if call == 0 {
			return []byte("RTNETLINK answers: Operation not permitted"), boom
		}
		// call 1 = recovery up; let it succeed.
		return nil, nil
	})
	stubReadMAC(t, func() string {
		t.Fatalf("readMACFile must NOT be invoked when down failed")
		return ""
	})

	err := ApplyMAC(context.Background(), wantMAC)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "down") {
		t.Errorf("error %q should mention 'down'", err)
	}
	if !errors.Is(err, boom) {
		t.Errorf("expected wrapped %v, got %v", boom, err)
	}
	// down (failing) + recovery up = 2 calls.
	if len(*got) != 2 {
		t.Errorf("expected 2 runCmd calls (down + recovery up), got %d", len(*got))
	}
}

func TestApplyMAC_AddressStepFails(t *testing.T) {
	const wantMAC = "aa:bb:cc:dd:ee:ff"
	boom := errors.New("address set rejected")
	got := stubRunCmd(t, func(call int, name string, args []string) ([]byte, error) {
		if call == 0 {
			return nil, nil
		}
		if call == 1 {
			return []byte("RTNETLINK answers: Invalid argument"), boom
		}
		// call 2 = recovery up; let it succeed.
		return nil, nil
	})
	stubReadMAC(t, func() string { return "" })

	err := ApplyMAC(context.Background(), wantMAC)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "address") {
		t.Errorf("error %q should mention 'address'", err)
	}
	// down + address (failing) + recovery up = 3 calls.
	if len(*got) != 3 {
		t.Errorf("expected 3 runCmd calls (down, address, recovery up), got %d", len(*got))
	}
}

func TestApplyMAC_UpStepFails(t *testing.T) {
	const wantMAC = "aa:bb:cc:dd:ee:ff"
	boom := errors.New("link up failed")
	got := stubRunCmd(t, func(call int, name string, args []string) ([]byte, error) {
		// down (0) + address (1) succeed; first up (2) fails; recovery up
		// (3) is best-effort — still also returns the boom but the caller
		// ignores it. Test the boom propagates and a recovery attempt was
		// issued.
		if call < 2 {
			return nil, nil
		}
		return []byte("RTNETLINK answers: Device or resource busy"), boom
	})
	stubReadMAC(t, func() string {
		t.Fatalf("readMACFile must NOT be invoked when up failed")
		return ""
	})

	err := ApplyMAC(context.Background(), wantMAC)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "up") {
		t.Errorf("error %q should mention 'up'", err)
	}
	// down + address + up (failing) + recovery up = 4 calls.
	if len(*got) != 4 {
		t.Errorf("expected 4 runCmd calls (down, address, up, recovery up), got %d", len(*got))
	}
}

func TestApplyMAC_RecoversUpOnAddressFailure(t *testing.T) {
	const wantMAC = "aa:bb:cc:dd:ee:ff"
	boom := errors.New("address set rejected")
	got := stubRunCmd(t, func(call int, name string, args []string) ([]byte, error) {
		// call 0 = down (ok); call 1 = address (fails); call 2 = recovery up.
		if call == 1 {
			return []byte("RTNETLINK answers: Invalid argument"), boom
		}
		return nil, nil
	})
	stubReadMAC(t, func() string {
		t.Fatalf("readMACFile must NOT be invoked when address failed")
		return ""
	})

	err := ApplyMAC(context.Background(), wantMAC)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("returned error must wrap the original boom: %v", err)
	}
	// 3 invocations total: down, address (failing), recovery up.
	if len(*got) != 3 {
		t.Fatalf("expected 3 runCmd calls (down, address, recovery up), got %d: %+v", len(*got), *got)
	}
	last := (*got)[len(*got)-1]
	wantArgs := []string{"link", "set", "dev", "eth0", "up"}
	if last.Name != "/bin/ip" || !reflect.DeepEqual(last.Args, wantArgs) {
		t.Errorf("last invocation = %+v, want recovery up %v", last, wantArgs)
	}
}

func TestApplyMAC_VerifyMismatch(t *testing.T) {
	const wantMAC = "aa:bb:cc:dd:ee:ff"
	stubRunCmd(t, func(call int, name string, args []string) ([]byte, error) { return nil, nil })
	stubReadMAC(t, func() string { return "11:22:33:44:55:66" })

	// Use a short context so verify gives up promptly without waiting
	// for the 2s default deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	err := ApplyMAC(ctx, wantMAC)
	if err == nil {
		t.Fatal("expected verify error")
	}
	if !strings.Contains(err.Error(), "verify") {
		t.Errorf("error %q should mention 'verify'", err)
	}
}
