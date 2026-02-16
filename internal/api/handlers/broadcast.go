// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/sse"
)

// Broadcaster publishes service updates to SSE clients.
type Broadcaster struct {
	hub *sse.Hub
}

func NewBroadcaster(hub *sse.Hub) *Broadcaster {
	return &Broadcaster{hub: hub}
}

func (b *Broadcaster) Publish(health models.ServiceHealth) {
	if b == nil || b.hub == nil {
		return
	}
	b.hub.Publish(EncodeHealthAsSSE(health))
}
