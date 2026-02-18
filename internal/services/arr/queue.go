// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"fmt"
	"strings"
)

type QueueDeleteOptions struct {
	RemoveFromClient bool
	Blocklist        bool
	SkipRedownload   bool
	ChangeCategory   bool
}

func BuildQueueDeleteURL(baseURL, queueID string, opts QueueDeleteOptions) string {
	baseURL = strings.TrimRight(baseURL, "/")

	deleteURL := fmt.Sprintf("%s/api/v3/queue/%s?removeFromClient=%t&blocklist=%t&skipRedownload=%t",
		baseURL,
		queueID,
		opts.RemoveFromClient,
		opts.Blocklist,
		opts.SkipRedownload,
	)

	if opts.ChangeCategory {
		deleteURL += "&changeCategory=true"
	}

	return deleteURL
}
