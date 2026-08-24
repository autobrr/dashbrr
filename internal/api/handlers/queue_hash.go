// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"fmt"
	"strings"

	"github.com/autobrr/dashbrr/internal/types"
)

// QueueRecordWrapper is a common wrapper for queue records
type QueueRecordWrapper struct {
	ID     int
	Title  string
	Status string
	Size   int64
}

func wrapQueueRecords[T any](records []T, toWrapper func(T) QueueRecordWrapper) []QueueRecordWrapper {
	if len(records) == 0 {
		return nil
	}

	result := make([]QueueRecordWrapper, len(records))
	for i, record := range records {
		result[i] = toWrapper(record)
	}
	return result
}

// wrapRadarrQueue converts RadarrQueueResponse to slice of QueueRecordWrapper
func wrapRadarrQueue(queue *types.RadarrQueueResponse) []QueueRecordWrapper {
	if queue == nil {
		return nil
	}

	return wrapQueueRecords(queue.Records, func(record types.RadarrQueueRecord) QueueRecordWrapper {
		return QueueRecordWrapper{
			ID:     record.ID,
			Title:  record.Title,
			Status: record.Status,
			Size:   record.Size,
		}
	})
}

// wrapLidarrQueue converts LidarrQueueResponse to slice of QueueRecordWrapper.
func wrapLidarrQueue(queue *types.LidarrQueueResponse) []QueueRecordWrapper {
	if queue == nil {
		return nil
	}

	return wrapQueueRecords(queue.Records, func(record types.LidarrQueueItem) QueueRecordWrapper {
		return QueueRecordWrapper{
			ID:     record.ID,
			Title:  record.Title,
			Status: record.Status,
			Size:   record.Size,
		}
	})
}

// wrapReadarrQueue converts ReadarrQueueResponse to slice of QueueRecordWrapper.
func wrapReadarrQueue(queue *types.ReadarrQueueResponse) []QueueRecordWrapper {
	if queue == nil {
		return nil
	}

	return wrapQueueRecords(queue.Records, func(record types.ReadarrQueueItem) QueueRecordWrapper {
		return QueueRecordWrapper{
			ID:     record.ID,
			Title:  record.Title,
			Status: record.Status,
			Size:   record.Size,
		}
	})
}

// wrapSonarrQueue converts SonarrQueueResponse to slice of QueueRecordWrapper
func wrapSonarrQueue(queue *types.SonarrQueueResponse) []QueueRecordWrapper {
	if queue == nil {
		return nil
	}

	return wrapQueueRecords(queue.Records, func(record types.QueueRecord) QueueRecordWrapper {
		return QueueRecordWrapper{
			ID:     record.ID,
			Title:  record.Title,
			Status: record.Status,
			Size:   record.Size,
		}
	})
}

// wrapWhisparrQueue converts WhisparrQueueResponse to slice of QueueRecordWrapper.
func wrapWhisparrQueue(queue *types.WhisparrQueueResponse) []QueueRecordWrapper {
	if queue == nil {
		return nil
	}

	return wrapQueueRecords(queue.Records, func(record types.WhisparrQueueItem) QueueRecordWrapper {
		return QueueRecordWrapper{
			ID:     record.ID,
			Title:  record.Title,
			Status: record.Status,
			Size:   record.Size,
		}
	})
}

// generateQueueHash creates a hash string from queue records
func generateQueueHash(records []QueueRecordWrapper) string {
	if len(records) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, record := range records {
		fmt.Fprintf(&sb, "%d:%s:%s:%d,",
			record.ID,
			record.Title,
			record.Status,
			record.Size)
	}
	return sb.String()
}

// detectQueueChanges determines the type of change in a queue
func detectQueueChanges(oldHash, newHash string) string {
	if oldHash == "" {
		return "initial_queue"
	}

	oldRecords := strings.Split(oldHash, ",")
	newRecords := strings.Split(newHash, ",")

	if len(oldRecords) < len(newRecords) {
		return "download_added"
	} else if len(oldRecords) > len(newRecords) {
		return "download_completed"
	}

	return "download_updated"
}
