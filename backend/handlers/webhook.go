// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

// TriggerWebhookArrs handles the webhook trigger for ARRs
func TriggerWebhookArrs(w http.ResponseWriter, r *http.Request) {
	apiKey := r.URL.Query().Get("apikey")
	if apiKey == "" {
		log.Warn().Msg("API key is required for ARRs webhook")
		http.Error(w, "API key is required", http.StatusBadRequest)
		return
	}

	// TODO: Implement ARRs update logic
	log.Info().Msg("ARRs update triggered")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ARRs update triggered"))
}

// TriggerWebhookLists handles the webhook trigger for Lists
func TriggerWebhookLists(w http.ResponseWriter, r *http.Request) {
	apiKey := r.URL.Query().Get("apikey")
	if apiKey == "" {
		log.Warn().Msg("API key is required for Lists webhook")
		http.Error(w, "API key is required", http.StatusBadRequest)
		return
	}

	// TODO: Implement Lists update logic
	log.Info().Msg("Lists update triggered")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Lists update triggered"))
}

// TriggerWebhookAll handles the webhook trigger for all updates
func TriggerWebhookAll(w http.ResponseWriter, r *http.Request) {
	apiKey := r.URL.Query().Get("apikey")
	if apiKey == "" {
		log.Warn().Msg("API key is required for all updates webhook")
		http.Error(w, "API key is required", http.StatusBadRequest)
		return
	}

	// TODO: Implement all updates logic
	log.Info().Msg("All updates triggered")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("All updates triggered"))
}
