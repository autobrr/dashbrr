// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"sync"

	"github.com/rs/zerolog/log"
)

func compareAndLogArrQueueChanges(
	lastHash map[string]string,
	lastHashMu *sync.Mutex,
	serviceName, instanceID string,
	totalRecords int,
	records []QueueRecordWrapper,
) {
	lastHashMu.Lock()
	defer lastHashMu.Unlock()

	currentHash := generateQueueHash(records)
	previousHash := lastHash[instanceID]
	if currentHash == previousHash {
		return
	}

	log.Debug().
		Str("instanceId", instanceID).
		Int("totalRecords", totalRecords).
		Str("change", detectQueueChanges(previousHash, currentHash)).
		Msgf("[%s] Queue changed", serviceName)

	lastHash[instanceID] = currentHash
}
