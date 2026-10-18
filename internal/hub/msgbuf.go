package hub

import (
	"sync"
	"time"
)

// MsgBuffers keeps the last N outbound envelopes per conversation so
// sync.request can be answered from memory with zero database reads.
//
// Unlike hub state, the buffers are read from client goroutines and therefore
// carry their own RWMutex.
type MsgBuffers struct {
	mu   sync.RWMutex
	m    map[string]*convBuf
	size int
	ttl  time.Duration
}

type convBuf struct {
	items    []Envelope
	lastUsed time.Time
}

// NewMsgBuffers creates a buffer set holding size envelopes per conversation.
func NewMsgBuffers(size int) *MsgBuffers {
	if size <= 0 {
		size = 50
	}
	return &MsgBuffers{m: make(map[string]*convBuf, 128), size: size, ttl: 30 * time.Minute}
}

// Append records an outbound envelope.
func (b *MsgBuffers) Append(convID string, e Envelope) {
	if convID == "" {
		return
	}
	if e.TS == 0 {
		e.TS = nowMS()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	cb, ok := b.m[convID]
	if !ok {
		cb = &convBuf{items: make([]Envelope, 0, b.size)}
		b.m[convID] = cb
	}
	cb.items = append(cb.items, e)
	if len(cb.items) > b.size {
		cb.items = append(cb.items[:0], cb.items[len(cb.items)-b.size:]...)
	}
	cb.lastUsed = time.Now()
}

// Since returns buffered envelopes newer than sinceMS. The boolean is false
// when the buffer cannot answer the request (no entries, or the requested
// timestamp is older than the oldest retained envelope) and the caller must
// fall back to the database.
func (b *MsgBuffers) Since(convID string, sinceMS int64) ([]Envelope, bool) {
	if convID == "" {
		return nil, false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	cb, ok := b.m[convID]
	if !ok || len(cb.items) == 0 {
		return nil, false
	}
	oldest := cb.items[0].TS
	if sinceMS < oldest && len(cb.items) >= b.size {
		return nil, false
	}
	out := make([]Envelope, 0, len(cb.items))
	for _, e := range cb.items {
		if e.TS > sinceMS {
			out = append(out, e)
		}
	}
	return out, true
}

// Len reports the buffered envelope count of a conversation.
func (b *MsgBuffers) Len(convID string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if cb, ok := b.m[convID]; ok {
		return len(cb.items)
	}
	return 0
}

// GC drops conversations that have been idle for longer than the TTL.
func (b *MsgBuffers) GC() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	cutoff := time.Now().Add(-b.ttl)
	dropped := 0
	for id, cb := range b.m {
		if cb.lastUsed.Before(cutoff) {
			delete(b.m, id)
			dropped++
		}
	}
	return dropped
}

// Count reports how many conversations are buffered.
func (b *MsgBuffers) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.m)
}
