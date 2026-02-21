// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package sonarr

import (
	"context"
	"errors"
	"testing"

	"github.com/autobrr/dashbrr/internal/services/arr"
	"github.com/autobrr/dashbrr/internal/types"
)

func TestDeleteQueueItem_ValidationErrorsAreArrErrors(t *testing.T) {
	t.Parallel()

	service := &SonarrService{}

	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		wantMsg string
	}{
		{
			name:    "missing url",
			baseURL: "",
			apiKey:  "key",
			wantMsg: "URL is required",
		},
		{
			name:    "missing api key",
			baseURL: "http://localhost:8989",
			apiKey:  "",
			wantMsg: "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := service.DeleteQueueItem(
				context.Background(),
				tt.baseURL,
				tt.apiKey,
				"123",
				types.SonarrQueueDeleteOptions{},
			)

			var arrErr *arr.ErrArr
			if !errors.As(err, &arrErr) {
				t.Fatalf("expected *arr.ErrArr, got %T (%v)", err, err)
			}
			if arrErr.Service != "sonarr" || arrErr.Op != "delete_queue" {
				t.Fatalf("unexpected arr error fields: %+v", arrErr)
			}
			if arrErr.Err == nil || arrErr.Err.Error() != tt.wantMsg {
				t.Fatalf("unexpected validation message: got %v want %q", arrErr.Err, tt.wantMsg)
			}
		})
	}
}
