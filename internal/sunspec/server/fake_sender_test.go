package server

import (
	"context"
	"sync"
)

func boolPtr(b bool) *bool { return &b }

// fakeSender stands in for *invdriver.Client in tests, recording every
// frame dispatched through the Model 123 write path so assertions can
// compare against the codec's expected bytes.
type fakeSender struct {
	mu   sync.Mutex
	sent []sentFrame
	err  error // when non-nil, Send returns it and records nothing
}

type sentFrame struct {
	uid   string
	frame []byte
}

func (f *fakeSender) Send(_ context.Context, uid string, frame []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentFrame{uid: uid, frame: append([]byte(nil), frame...)})
	return nil
}

// frames returns a copy of every recorded send, in order.
func (f *fakeSender) frames() []sentFrame {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentFrame(nil), f.sent...)
}

// frameFor returns the most recent frame sent to uid, or nil if none.
func (f *fakeSender) frameFor(uid string) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.sent) - 1; i >= 0; i-- {
		if f.sent[i].uid == uid {
			return f.sent[i].frame
		}
	}
	return nil
}
