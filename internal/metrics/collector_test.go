// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package metrics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/autobrr/dashbrr/internal/models"
)

func fakeServices(services ...models.ServiceConfiguration) func(context.Context) ([]models.ServiceConfiguration, error) {
	return func(context.Context) ([]models.ServiceConfiguration, error) {
		return services, nil
	}
}

func fakeLatest(snapshots ...models.ServiceHealth) func(string) (models.ServiceHealth, bool) {
	byID := make(map[string]models.ServiceHealth)
	for _, s := range snapshots {
		byID[s.ServiceID] = s
	}
	return func(id string) (models.ServiceHealth, bool) {
		s, ok := byID[id]
		return s, ok
	}
}

func TestCollectorAllMetrics(t *testing.T) {
	c := NewCollector(
		fakeServices(models.ServiceConfiguration{InstanceID: "radarr-1", DisplayName: "Movies"}),
		fakeLatest(models.ServiceHealth{
			ServiceID:       "radarr-1",
			Status:          "warning",
			Message:         "Indexers unavailable",
			ResponseTime:    250,
			Version:         "5.2.6",
			UpdateAvailable: true,
			LastChecked:     time.Unix(1700000000, 0),
		}),
	)

	want := `
# HELP dashbrr_info Version of dashbrr. The value is always 1.
# TYPE dashbrr_info gauge
dashbrr_info{version="dev"} 1
# HELP dashbrr_service_info Version of the service. The value is always 1.
# TYPE dashbrr_service_info gauge
dashbrr_service_info{instance_id="radarr-1",instance_name="Movies",service_type="radarr",version="5.2.6"} 1
# HELP dashbrr_service_last_check_timestamp_seconds Unix time of the last health check.
# TYPE dashbrr_service_last_check_timestamp_seconds gauge
dashbrr_service_last_check_timestamp_seconds{instance_id="radarr-1",instance_name="Movies",service_type="radarr"} 1.7e+09
# HELP dashbrr_service_response_time_seconds Response time of the last health check, in seconds.
# TYPE dashbrr_service_response_time_seconds gauge
dashbrr_service_response_time_seconds{instance_id="radarr-1",instance_name="Movies",service_type="radarr"} 0.25
# HELP dashbrr_service_status Service status after the last health check. The series of the current status is 1, the others are 0.
# TYPE dashbrr_service_status gauge
dashbrr_service_status{instance_id="radarr-1",instance_name="Movies",service_type="radarr",status="error"} 0
dashbrr_service_status{instance_id="radarr-1",instance_name="Movies",service_type="radarr",status="offline"} 0
dashbrr_service_status{instance_id="radarr-1",instance_name="Movies",service_type="radarr",status="online"} 0
dashbrr_service_status{instance_id="radarr-1",instance_name="Movies",service_type="radarr",status="pending"} 0
dashbrr_service_status{instance_id="radarr-1",instance_name="Movies",service_type="radarr",status="unknown"} 0
dashbrr_service_status{instance_id="radarr-1",instance_name="Movies",service_type="radarr",status="warning"} 1
# HELP dashbrr_service_up Whether the service is up (1) or down (0). The online and warning statuses are up.
# TYPE dashbrr_service_up gauge
dashbrr_service_up{instance_id="radarr-1",instance_name="Movies",service_type="radarr"} 1
# HELP dashbrr_service_update_available Whether an update is available for the service (1) or not (0).
# TYPE dashbrr_service_update_available gauge
dashbrr_service_update_available{instance_id="radarr-1",instance_name="Movies",service_type="radarr"} 1
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(want)); err != nil {
		t.Error(err)
	}
}

func TestCollectorUpAndStatusForEachStatus(t *testing.T) {
	tests := []struct {
		status string
		up     string
	}{
		{"online", "1"},
		{"warning", "1"},
		{"offline", "0"},
		{"error", "0"},
		{"pending", "0"},
		{"unknown", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			c := NewCollector(
				fakeServices(models.ServiceConfiguration{InstanceID: "sonarr-1", DisplayName: "Shows"}),
				fakeLatest(models.ServiceHealth{ServiceID: "sonarr-1", Status: tt.status}),
			)

			var want strings.Builder
			want.WriteString(`
# HELP dashbrr_service_up Whether the service is up (1) or down (0). The online and warning statuses are up.
# TYPE dashbrr_service_up gauge
dashbrr_service_up{instance_id="sonarr-1",instance_name="Shows",service_type="sonarr"} ` + tt.up + `
# HELP dashbrr_service_status Service status after the last health check. The series of the current status is 1, the others are 0.
# TYPE dashbrr_service_status gauge
`)
			for _, s := range []string{"error", "offline", "online", "pending", "unknown", "warning"} {
				value := "0"
				if s == tt.status {
					value = "1"
				}
				want.WriteString(`dashbrr_service_status{instance_id="sonarr-1",instance_name="Shows",service_type="sonarr",status="` + s + `"} ` + value + "\n")
			}

			if err := testutil.CollectAndCompare(c, strings.NewReader(want.String()), "dashbrr_service_up", "dashbrr_service_status"); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestCollectorSkipsServicesWithoutBothConfigAndSnapshot(t *testing.T) {
	c := NewCollector(
		// qui-1 has no snapshot yet.
		fakeServices(models.ServiceConfiguration{InstanceID: "qui-1", DisplayName: "qui"}),
		// lidarr-1 was deleted, but the snapshot keeps it.
		fakeLatest(models.ServiceHealth{ServiceID: "lidarr-1", Status: "online"}),
	)

	if got := testutil.CollectAndCount(c); got != 1 {
		t.Errorf("collected %d series, want only dashbrr_info", got)
	}
}

func TestCollectorFailsWhenServicesFail(t *testing.T) {
	c := NewCollector(
		func(context.Context) ([]models.ServiceConfiguration, error) {
			return nil, errors.New("database is locked")
		},
		fakeLatest(),
	)

	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	if _, err := reg.Gather(); err == nil {
		t.Error("Gather() error = nil, want the error from services")
	}
}
