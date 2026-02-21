// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package sse

import (
	"context"
	"sync"
	"sync/atomic"
)

// Hub fan-outs already-encoded SSE "data: ...\n\n" payloads to subscribers.
// Non-blocking: slow subscribers will drop messages.
type Hub struct {
	nextID atomic.Int64

	mu     sync.RWMutex
	subs   map[int64]chan []byte
	closed atomic.Bool
}

func NewHub() *Hub {
	return &Hub{
		subs: make(map[int64]chan []byte),
	}
}

func (h *Hub) Subscribe(ctx context.Context, buffer int) (<-chan []byte, func()) {
	if buffer <= 0 {
		buffer = 64
	}

	id := h.nextID.Add(1)
	ch := make(chan []byte, buffer)

	h.mu.Lock()
	if h.closed.Load() {
		h.mu.Unlock()
		close(ch)
		return ch, func() {}
	}
	h.subs[id] = ch
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		sub, ok := h.subs[id]
		if ok {
			delete(h.subs, id)
			close(sub)
		}
		h.mu.Unlock()
	}

	go func() {
		<-ctx.Done()
		unsub()
	}()

	return ch, unsub
}

func (h *Hub) Publish(payload []byte) {
	if h.closed.Load() {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, ch := range h.subs {
		select {
		case ch <- payload:
		default:
			// drop for slow consumer
		}
	}
}

func (h *Hub) Close() {
	if !h.closed.CompareAndSwap(false, true) {
		return
	}

	h.mu.Lock()
	for id, ch := range h.subs {
		delete(h.subs, id)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}
