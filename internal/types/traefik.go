// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package types

type TraefikSection struct {
	Total    int `json:"total"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

type TraefikSchemeOverview struct {
	Routers     *TraefikSection `json:"routers,omitempty"`
	Services    *TraefikSection `json:"services,omitempty"`
	Middlewares *TraefikSection `json:"middlewares,omitempty"`
}

type TraefikFeatures struct {
	Tracing   string `json:"tracing"`
	Metrics   string `json:"metrics"`
	AccessLog bool   `json:"accessLog"`
}

type TraefikOverviewResponse struct {
	HTTP      TraefikSchemeOverview `json:"http"`
	TCP       TraefikSchemeOverview `json:"tcp"`
	UDP       TraefikSchemeOverview `json:"udp"`
	Features  TraefikFeatures       `json:"features,omitempty"`
	Providers []string              `json:"providers,omitempty"`
}

type TraefikRouter struct {
	Name        string   `json:"name"`
	Provider    string   `json:"provider"`
	Status      string   `json:"status"`
	Rule        string   `json:"rule"`
	Service     string   `json:"service"`
	EntryPoints []string `json:"entryPoints,omitempty"`
	Middlewares []string `json:"middlewares,omitempty"`
	Using       []string `json:"using,omitempty"`
}

type TraefikVersionResponse struct {
	Version string `json:"version"`
}

type TraefikSummaryResponse struct {
	Overview     TraefikOverviewResponse    `json:"overview"`
	IssueRouters []TraefikRouter            `json:"issueRouters"`
	Certificates *TraefikCertificateSummary `json:"certificates,omitempty"`
}

type TraefikCertificate struct {
	CommonName       string   `json:"commonName,omitempty"`
	Serial           string   `json:"serial,omitempty"`
	SANs             []string `json:"sans,omitempty"`
	NotAfter         string   `json:"notAfter"`
	NotAfterUnix     int64    `json:"notAfterUnix"`
	ExpiresInSeconds int64    `json:"expiresInSeconds"`
	Status           string   `json:"status"`
}

type TraefikCertificateSummary struct {
	Total               int                  `json:"total"`
	Expired             int                  `json:"expired"`
	ExpiringSoon        int                  `json:"expiringSoon"`
	NextExpiry          string               `json:"nextExpiry,omitempty"`
	NextExpiryUnix      int64                `json:"nextExpiryUnix,omitempty"`
	NextExpiryInSeconds int64                `json:"nextExpiryInSeconds,omitempty"`
	MetricsURL          string               `json:"metricsUrl,omitempty"`
	MetricName          string               `json:"metricName,omitempty"`
	Certificates        []TraefikCertificate `json:"certificates,omitempty"`
}
