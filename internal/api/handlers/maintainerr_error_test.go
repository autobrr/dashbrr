// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/autobrr/dashbrr/internal/services/maintainerr"
)

func TestDetermineErrorResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name: "maintainerr missing url",
			err: &maintainerr.ErrMaintainerr{
				Op:  "get_collections",
				Err: maintainerr.ErrURLRequired,
			},
			wantStatus: http.StatusBadRequest,
			wantMsg:    "maintainerr get_collections: URL is required",
		},
		{
			name: "maintainerr missing api key",
			err: &maintainerr.ErrMaintainerr{
				Op:  "get_collections",
				Err: maintainerr.ErrAPIKeyRequired,
			},
			wantStatus: http.StatusBadRequest,
			wantMsg:    "maintainerr get_collections: API key is required",
		},
		{
			name: "maintainerr upstream unauthorized",
			err: &maintainerr.ErrMaintainerr{
				Op:       "get_collections",
				HttpCode: http.StatusUnauthorized,
			},
			wantStatus: http.StatusBadGateway,
			wantMsg:    "Invalid API key",
		},
		{
			name:       "deadline exceeded",
			err:        context.DeadlineExceeded,
			wantStatus: http.StatusGatewayTimeout,
			wantMsg:    "Request timed out",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotStatus, gotMsg := determineErrorResponse(tt.err)
			if gotStatus != tt.wantStatus {
				t.Fatalf("determineErrorResponse status = %d, want %d", gotStatus, tt.wantStatus)
			}
			if gotMsg != tt.wantMsg {
				t.Fatalf("determineErrorResponse message = %q, want %q", gotMsg, tt.wantMsg)
			}
		})
	}
}
