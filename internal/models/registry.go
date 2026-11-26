// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

// ServiceCreator is responsible for creating service instances
type ServiceCreator interface {
	CreateService(serviceType string) ServiceHealthChecker
}
