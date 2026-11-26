// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package core

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var ErrInvalidServiceURL = errors.New("invalid service URL")

func RequestURL(base string, elems ...string) (string, error) {
	return RequestURLQuery(base, nil, elems...)
}

func RequestURLQuery(base string, query url.Values, elems ...string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidServiceURL, RedactURLError(err))
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("%w: %q needs a scheme and a host", ErrInvalidServiceURL, RedactURL(u.String()))
	}

	for _, elem := range elems {
		if _, err := url.PathUnescape(elem); err != nil {
			return "", fmt.Errorf("%w: path %q: %w", ErrInvalidServiceURL, elem, err)
		}
		for segment := range strings.SplitSeq(elem, "/") {
			if segment == "." || segment == ".." {
				return "", fmt.Errorf("%w: path %q holds a dot segment", ErrInvalidServiceURL, elem)
			}
		}
	}

	endpoint := u.JoinPath(elems...)
	if len(query) > 0 {
		merged := endpoint.Query()
		for key, values := range query {
			for _, value := range values {
				merged.Add(key, value)
			}
		}
		endpoint.RawQuery = merged.Encode()
	}

	return endpoint.String(), nil
}
