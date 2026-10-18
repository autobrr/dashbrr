// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package metrics

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

// Start binds addr and serves the metrics on it in the background. It returns
// the bind error, so that the caller can keep dashbrr running without metrics.
func Start(addr string, collector prometheus.Collector, users map[string]string) (*http.Server, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	log.Info().Str("address", listener.Addr().String()).Bool("basic_auth", len(users) > 0).Msg("Starting metrics server")

	srv := &http.Server{
		Handler:           Handler(collector, users),
		ReadHeaderTimeout: 15 * time.Second,
	}
	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("Metrics server stopped")
		}
	}()

	return srv, nil
}

// Handler serves GET /metrics with the Go, process, and dashbrr metrics.
// A non-empty users map turns on basic auth.
func Handler(collector prometheus.Collector, users map[string]string) http.Handler {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collector,
	)

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{EnableOpenMetrics: true}))

	if len(users) == 0 {
		return mux
	}
	return basicAuth(users, mux)
}

func basicAuth(users map[string]string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, password, ok := r.BasicAuth(); ok {
			for wantUser, wantPassword := range users {
				userOK := subtle.ConstantTimeCompare([]byte(user), []byte(wantUser))
				passwordOK := subtle.ConstantTimeCompare([]byte(password), []byte(wantPassword))
				if userOK&passwordOK == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	})
}
