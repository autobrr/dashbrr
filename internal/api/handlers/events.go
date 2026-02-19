// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
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

	flush := func() {
		if f, ok := c.Writer.(interface{ Flush() }); ok {
			f.Flush()
		}
	}
	write := func(payload []byte) bool {
		if _, err := c.Writer.Write(payload); err != nil {
			if !isExpectedSSEWriteError(ctx, err) {
				log.Warn().
					Uint64("client_id", clientID).
					Err(err).
					Msg("SSE write failed")
			}
			return false
		}
		flush()
		return true
	}
	writeString := func(payload string) bool {
		if _, err := c.Writer.WriteString(payload); err != nil {
			if !isExpectedSSEWriteError(ctx, err) {
				log.Warn().
					Uint64("client_id", clientID).
					Err(err).
					Msg("SSE write failed")
			}
			return false
		}
		flush()
		return true
	}

	if !writeString("retry: 5000\n\n") {
		return
	}

	for _, payload := range h.bc.Snapshot() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !write(payload) {
			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepAlive.C:
			// comment line, ignored by EventSource but keeps proxies from buffering.
			if !writeString(": keepalive\n\n") {
				return
			}
		case payload, ok := <-sub:
			if !ok {
				return
			}
			if !write(payload) {
				return
			}
		}
	}
}

func isExpectedSSEWriteError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx.Err() != nil {
		return true
	}
	return errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET)
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
