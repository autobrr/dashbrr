// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"errors"
	"fmt"
)

var ErrServiceNotConfigured = errors.New("service not configured")

type ServiceNotConfiguredError struct {
	Service string
}

func (e *ServiceNotConfiguredError) Error() string {
	if e == nil || e.Service == "" {
		return ErrServiceNotConfigured.Error()
	}
	return fmt.Sprintf("%s is not configured", e.Service)
}

func (e *ServiceNotConfiguredError) Unwrap() error {
	return ErrServiceNotConfigured
}

func NewServiceNotConfigured(service string) error {
	return &ServiceNotConfiguredError{Service: service}
}
