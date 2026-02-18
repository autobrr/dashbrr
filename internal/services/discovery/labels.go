// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package discovery

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

type parsedDiscoveryLabels struct {
	serviceType string
	url         string
	apiKey      string
	displayName string
	enabled     bool
}

func parseDiscoveryLabels(labels map[string]string) (*parsedDiscoveryLabels, error) {
	serviceType := labels[GetLabelKey(labelTypeKey)]
	if serviceType == "" {
		return nil, fmt.Errorf("service type label not found")
	}

	url := labels[GetLabelKey(labelURLKey)]
	if url == "" {
		return nil, fmt.Errorf("service URL label not found")
	}

	apiKey, err := resolveEnvVar(labels[GetLabelKey(labelAPIKeyKey)])
	if err != nil {
		return nil, err
	}

	displayName := labels[GetLabelKey(labelNameKey)]
	if displayName == "" {
		displayName = titleServiceType(serviceType)
	}

	enabled := true
	if v, ok := labels[GetLabelKey(labelEnabledKey)]; ok {
		enabled = v != "false"
	}

	return &parsedDiscoveryLabels{
		serviceType: serviceType,
		url:         url,
		apiKey:      apiKey,
		displayName: displayName,
		enabled:     enabled,
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
