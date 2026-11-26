// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package discovery

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/autobrr/dashbrr/internal/services/core"
)

type parsedDiscoveryLabels struct {
	serviceType string
	url         string
	apiKey      string
	displayName string
	accessURL   string
}

// discoveryDisabled reports whether the labels turn discovery off for a
// service. A caller checks it before it validates the other labels, so a
// disabled service with incomplete labels logs no warning.
func discoveryDisabled(labels map[string]string) bool {
	return labels[GetLabelKey(labelEnabledKey)] == "false"
}

func parseDiscoveryLabels(labels map[string]string) (*parsedDiscoveryLabels, error) {
	serviceType := labels[GetLabelKey(labelTypeKey)]
	if serviceType == "" {
		return nil, fmt.Errorf("service type label not found")
	}

	url := strings.TrimSpace(labels[GetLabelKey(labelURLKey)])
	if url == "" {
		return nil, fmt.Errorf("service URL label not found")
	}
	if err := core.ValidateServiceURL(url); err != nil {
		return nil, fmt.Errorf("%s: %w", GetLabelKey(labelURLKey), err)
	}
	accessURL := strings.TrimSpace(labels[GetLabelKey(labelAccessURLKey)])
	if accessURL != "" {
		if err := core.ValidateServiceURL(accessURL); err != nil {
			return nil, fmt.Errorf("%s: %w", GetLabelKey(labelAccessURLKey), err)
		}
	}

	apiKey, err := resolveEnvVar(labels[GetLabelKey(labelAPIKeyKey)])
	if err != nil {
		return nil, err
	}

	displayName := labels[GetLabelKey(labelNameKey)]
	if displayName == "" {
		displayName = titleServiceType(serviceType)
	}

	return &parsedDiscoveryLabels{
		serviceType: serviceType,
		url:         url,
		apiKey:      apiKey,
		displayName: displayName,
		accessURL:   accessURL,
	}, nil
}

func resolveEnvVar(raw string) (string, error) {
	if !strings.HasPrefix(raw, "${") || !strings.HasSuffix(raw, "}") {
		return raw, nil
	}

	envVar := strings.TrimSuffix(strings.TrimPrefix(raw, "${"), "}")
	v := os.Getenv(envVar)
	if v == "" {
		return "", fmt.Errorf("environment variable %s not set", envVar)
	}
	return v, nil
}

func titleServiceType(serviceType string) string {
	if serviceType == "" {
		return ""
	}
	r := []rune(serviceType)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
