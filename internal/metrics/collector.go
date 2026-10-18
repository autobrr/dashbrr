// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

// Package metrics serves Prometheus metrics for the configured services.
package metrics

import (
	"context"
	"slices"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/autobrr/dashbrr/internal/buildinfo"
	"github.com/autobrr/dashbrr/internal/models"
)

// statuses holds each service status that has a dashbrr_service_status series.
var statuses = []string{"online", "warning", "offline", "error", "pending", "unknown"}

var serviceLabels = []string{"instance_id", "instance_name", "service_type"}

// Collector builds the service metrics on each scrape from the configured
// services and the last health of each service. A scrape sends no request to
// a service.
type Collector struct {
	services func(context.Context) ([]models.ServiceConfiguration, error)
	latest   func(instanceID string) (models.ServiceHealth, bool)

	info            *prometheus.Desc
	up              *prometheus.Desc
	status          *prometheus.Desc
	responseTime    *prometheus.Desc
	updateAvailable *prometheus.Desc
	serviceInfo     *prometheus.Desc
	lastCheck       *prometheus.Desc
}

// NewCollector returns a Collector. services lists the configured services,
// and latest returns the last health of one service.
func NewCollector(
	services func(context.Context) ([]models.ServiceConfiguration, error),
	latest func(instanceID string) (models.ServiceHealth, bool),
) *Collector {
	return &Collector{
		services: services,
		latest:   latest,
		info: prometheus.NewDesc("dashbrr_info",
			"Version of dashbrr. The value is always 1.",
			[]string{"version"}, nil),
		up: prometheus.NewDesc("dashbrr_service_up",
			"Whether the service is up (1) or down (0). The online and warning statuses are up.",
			serviceLabels, nil),
		status: prometheus.NewDesc("dashbrr_service_status",
			"Service status after the last health check. The series of the current status is 1, the others are 0.",
			slices.Concat(serviceLabels, []string{"status"}), nil),
		responseTime: prometheus.NewDesc("dashbrr_service_response_time_seconds",
			"Response time of the last health check, in seconds.",
			serviceLabels, nil),
		updateAvailable: prometheus.NewDesc("dashbrr_service_update_available",
			"Whether an update is available for the service (1) or not (0).",
			serviceLabels, nil),
		serviceInfo: prometheus.NewDesc("dashbrr_service_info",
			"Version of the service. The value is always 1.",
			slices.Concat(serviceLabels, []string{"version"}), nil),
		lastCheck: prometheus.NewDesc("dashbrr_service_last_check_timestamp_seconds",
			"Unix time of the last health check.",
			serviceLabels, nil),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.info
	ch <- c.up
	ch <- c.status
	ch <- c.responseTime
	ch <- c.updateAvailable
	ch <- c.serviceInfo
	ch <- c.lastCheck
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.info, prometheus.GaugeValue, 1, buildinfo.Version)

	services, err := c.services(context.Background())
	if err != nil {
		// The scrape answers HTTP 500, so Prometheus marks dashbrr as down.
		ch <- prometheus.NewInvalidMetric(c.up, err)
		return
	}

	for _, service := range services {
		health, ok := c.latest(service.InstanceID)
		if !ok {
			continue
		}

		serviceType, _ := models.ServiceTypeFromInstanceID(service.InstanceID)
		labels := []string{service.InstanceID, service.DisplayName, serviceType}
		gauge := func(desc *prometheus.Desc, value float64, extra ...string) {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, append(labels, extra...)...)
		}

		gauge(c.up, boolValue(health.Status == "online" || health.Status == "warning"))
		for _, status := range statuses {
			gauge(c.status, boolValue(health.Status == status), status)
		}
		gauge(c.responseTime, float64(health.ResponseTime)/1000)
		gauge(c.updateAvailable, boolValue(health.UpdateAvailable))
		gauge(c.serviceInfo, 1, health.Version)
		gauge(c.lastCheck, float64(health.LastChecked.Unix()))
	}
}

func boolValue(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
