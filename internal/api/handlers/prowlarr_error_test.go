// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/autobrr/dashbrr/internal/services/arr"
)

func TestStatusFromProwlarrError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "service not configured",
			err:  NewServiceNotConfigured("prowlarr"),
			want: http.StatusNotFound,
		},
		{
			name: "upstream unauthorized maps to bad gateway",
			err: &arr.ErrArr{
				Service:  "prowlarr",
				Op:       "get_indexers",
				HttpCode: http.StatusUnauthorized,
			},
			want: http.StatusBadGateway,
		},
		{
			name: "generic error",
			err:  errors.New("boom"),
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := statusFromProwlarrError(tt.err)
			if got != tt.want {
				t.Fatalf("statusFromProwlarrError() = %d, want %d", got, tt.want)
			}
		})
	}
}
