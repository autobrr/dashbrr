// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/sse"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

type EventsHandler struct {
	hub          *sse.Hub
	bc           *Broadcaster
	nextClientID atomic.Uint64
}

func NewEventsHandler(hub *sse.Hub, bc *Broadcaster) *EventsHandler {
	return &EventsHandler{hub: hub, bc: bc}
}

// Stream streams all published service updates.
// Contract: default SSE message events, each is JSON `models.ServiceHealth`.
func (h *EventsHandler) Stream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")

	ctx := c.Request.Context()
	sub, _ := h.hub.Subscribe(ctx, 128)
	clientID := h.nextClientID.Add(1)

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	log.Debug().
		Uint64("client_id", clientID).
		Int("subscribers", h.hub.SubscriberCount()).
		Msg("SSE client connected")
	defer log.Debug().
		Uint64("client_id", clientID).
		Int("subscribers", h.hub.SubscriberCount()).
		Msg("SSE client disconnected")

	flush := func() {
		if f, ok := c.Writer.(interface{ Flush() }); ok {
			f.Flush()
		}
	}

	_, _ = c.Writer.WriteString("retry: 5000\n\n")
	flush()

	for _, payload := range h.bc.Snapshot() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, _ = c.Writer.Write(payload)
		flush()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepAlive.C:
			// comment line, ignored by EventSource but keeps proxies from buffering.
			_, _ = c.Writer.WriteString(": keepalive\n\n")
			flush()
		case payload, ok := <-sub:
			if !ok {
				return
			}
			_, _ = c.Writer.Write(payload)
			flush()
		}
	}
}

func EncodeHealthAsSSE(health models.ServiceHealth) []byte {
	b, err := json.Marshal(health)
	if err != nil {
		// keep stream alive; send a minimal error payload
		fallback := fmt.Sprintf(`{"status":"error","message":"failed to encode event"}`)
		return []byte("data: " + fallback + "\n\n")
	}

	// SSE requires each line prefixed with "data: ". json.Marshal outputs single-line by default,
	// but handle any newlines defensively.
	s := string(b)
	s = strings.ReplaceAll(s, "\n", "\ndata: ")
	return []byte("data: " + s + "\n\n")
}
