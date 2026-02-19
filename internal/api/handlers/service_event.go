// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"regexp"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
)

var internalServiceEventMessagePattern = regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)+$`)

func classifyServiceEventType(health models.ServiceHealth) models.ServiceEventType {
	switch health.EventType {
	case models.ServiceEventInternal:
		return models.ServiceEventInternal
	case models.ServiceEventHealth:
		return models.ServiceEventHealth
	}

	if health.Message != "" && internalServiceEventMessagePattern.MatchString(health.Message) {
		return models.ServiceEventInternal
	}

	return models.ServiceEventHealth
}

func normalizeServiceEvent(health models.ServiceHealth) models.ServiceHealth {
	if health.LastChecked.IsZero() {
		health.LastChecked = time.Now()
	}

	health.EventType = classifyServiceEventType(health)
	return health
}

func isInternalServiceEvent(health models.ServiceHealth) bool {
	return classifyServiceEventType(health) == models.ServiceEventInternal
}

func shouldMergeHealthState(health models.ServiceHealth) bool {
	return !isInternalServiceEvent(health)
}

func publishInternalServiceUpdate(bc *Broadcaster, health models.ServiceHealth) {
	health.EventType = models.ServiceEventInternal
	bc.Publish(health)
}

func publishHealthServiceUpdate(bc *Broadcaster, health models.ServiceHealth) {
	health.EventType = models.ServiceEventHealth
	bc.Publish(health)
}
