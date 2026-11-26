// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"fmt"
	"net/url"
	"strconv"
)

func ValidateServiceURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: cannot be parsed", ErrInvalidServiceURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: must start with http:// or https://", ErrInvalidServiceURL)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("%w: has no host", ErrInvalidServiceURL)
	}
	if p := u.Port(); p != "" {
		if _, err := strconv.ParseUint(p, 10, 16); err != nil {
			return fmt.Errorf("%w: port must be at most 65535", ErrInvalidServiceURL)
		}
	}
	return nil
}
