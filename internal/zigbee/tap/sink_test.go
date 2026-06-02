package tap

import (
	"sync"
	"testing"
	"time"
)

type chunkSink struct {
	mu     sync.Mutex
	writes [][]byte
}

func (s *chunkSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := append([]byte(nil), p...)
	s.writes = append(s.writes, cp)
	return len(p), nil
}

func (s *chunkSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.writes)
}

func (s *chunkSink) waitFor(t *testing.T, n int, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.count() >= n {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return s.count() >= n
}
