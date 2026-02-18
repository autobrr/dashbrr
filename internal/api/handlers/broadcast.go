// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/autobrr/dashbrr/internal/sse"
)

var internalEventMessagePattern = regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)+$`)

// Broadcaster publishes service updates to SSE clients.
type Broadcaster struct {
	hub *sse.Hub

	mu     sync.RWMutex
	latest map[string]models.ServiceHealth
}

func NewBroadcaster(hub *sse.Hub) *Broadcaster {
	return &Broadcaster{
		hub:    hub,
		latest: make(map[string]models.ServiceHealth),
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
	prev, hasPrev := b.latest[health.ServiceID]
	if hasPrev {
		b.latest[health.ServiceID] = mergeHealthSnapshot(prev, health)
	} else {
		b.latest[health.ServiceID] = health
	}
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
		out = append(out, EncodeHealthAsSSE(b.latest[serviceID]))
	}

	return out
}

func mergeHealthSnapshot(prev, next models.ServiceHealth) models.ServiceHealth {
	merged := prev
	merged.ServiceID = next.ServiceID

	shouldMergeHealthState := next.Message != "" && !internalEventMessagePattern.MatchString(next.Message)

	if shouldMergeHealthState && next.Status != "" {
		merged.Status = next.Status
	}
	if shouldMergeHealthState && next.Message != "" {
		merged.Message = next.Message
	}
	if !next.LastChecked.IsZero() {
		merged.LastChecked = next.LastChecked
	} else if merged.LastChecked.IsZero() {
		merged.LastChecked = time.Now()
	}

	if next.Version != "" {
		merged.Version = next.Version
	}
	if shouldMergeHealthState {
		merged.ResponseTime = next.ResponseTime
		merged.UpdateAvailable = next.UpdateAvailable
	}

	merged.Stats = mergeHealthPayload(prev.Stats, next.Stats)
	merged.Details = mergeHealthPayload(prev.Details, next.Details)

	return merged
}

func mergeHealthPayload(current, incoming map[string]interface{}) map[string]interface{} {
	if incoming == nil {
		return current
	}
	if current == nil {
		return cloneMap(incoming)
	}

	merged := cloneMap(current)
	for key, nextValue := range incoming {
		prevValue, ok := current[key]
		if ok {
			prevMap, prevOK := prevValue.(map[string]interface{})
			nextMap, nextOK := nextValue.(map[string]interface{})
			if prevOK && nextOK {
				merged[key] = mergeHealthPayload(prevMap, nextMap)
				continue
			}
		}
		merged[key] = nextValue
	}

	return merged
}

func cloneMap(value map[string]interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}

	cloned := make(map[string]interface{}, len(value))
	for key, v := range value {
		if nested, ok := v.(map[string]interface{}); ok {
			cloned[key] = cloneMap(nested)
			continue
		}
		cloned[key] = v
	}

	return cloned
}
