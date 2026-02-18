// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"sort"
	"sync"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/sse"
)

// Broadcaster publishes service updates to SSE clients.
type Broadcaster struct {
	hub *sse.Hub

	mu     sync.RWMutex
	latest map[string][]byte
}

func NewBroadcaster(hub *sse.Hub) *Broadcaster {
	return &Broadcaster{
		hub:    hub,
		latest: make(map[string][]byte),
	}
}

func (b *Broadcaster) Publish(health models.ServiceHealth) {
	if b == nil || b.hub == nil {
		return
	}

	payload := EncodeHealthAsSSE(health)
	b.hub.Publish(payload)

	if health.ServiceID == "" {
		return
	}

	b.mu.Lock()
	b.latest[health.ServiceID] = append([]byte(nil), payload...)
	b.mu.Unlock()
}

// Snapshot returns last known payloads per service for initial SSE replay.
func (b *Broadcaster) Snapshot() [][]byte {
	if b == nil {
		return nil
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.latest) == 0 {
		return nil
	}

	keys := make([]string, 0, len(b.latest))
	for serviceID := range b.latest {
		keys = append(keys, serviceID)
	}
	sort.Strings(keys)

	out := make([][]byte, 0, len(keys))
	for _, serviceID := range keys {
		out = append(out, append([]byte(nil), b.latest[serviceID]...))
	}

	return out
}
