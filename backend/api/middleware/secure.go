// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// SecureConfig holds configuration for secure headers
type SecureConfig struct {
	CSPEnabled            bool
	CSPDefaultSrc         []string
	CSPScriptSrc          []string
	CSPStyleSrc           []string
	CSPImgSrc             []string
	CSPConnectSrc         []string
	CSPFontSrc            []string
	CSPObjectSrc          []string
	CSPMediaSrc           []string
	CSPFrameSrc           []string
	HSTSEnabled           bool
	HSTSMaxAge            int
	HSTSIncludeSubdomains bool
	HSTSPreload           bool
	FrameGuardEnabled     bool
	FrameGuardAction      string // DENY, SAMEORIGIN
	ContentTypeNosniff    bool
	XSSProtection         bool
	XSSProtectionMode     string // "0", "1", "1; mode=block"
	ReferrerPolicy        string
}

// DefaultSecureConfig returns the default secure configuration
func DefaultSecureConfig() *SecureConfig {
	return &SecureConfig{
		CSPEnabled:            true,
		CSPDefaultSrc:         []string{"'self'"},
		CSPScriptSrc:          []string{"'self'", "'unsafe-inline'", "'unsafe-eval'"},
		CSPStyleSrc:           []string{"'self'", "'unsafe-inline'"},
		CSPImgSrc:             []string{"'self'", "data:", "https:"},
		CSPConnectSrc:         []string{"'self'"},
		CSPFontSrc:            []string{"'self'"},
		CSPObjectSrc:          []string{"'none'"},
		CSPMediaSrc:           []string{"'self'"},
		CSPFrameSrc:           []string{"'none'"},
		HSTSEnabled:           true,
		HSTSMaxAge:            31536000, // 1 year
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
		FrameGuardEnabled:     true,
		FrameGuardAction:      "DENY",
		ContentTypeNosniff:    true,
		XSSProtection:         true,
		XSSProtectionMode:     "1; mode=block",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
	}
}

// buildCSPHeader builds the Content-Security-Policy header value
func (c *SecureConfig) buildCSPHeader() string {
	if !c.CSPEnabled {
		return ""
	}

	csp := ""
	if len(c.CSPDefaultSrc) > 0 {
		csp += "default-src " + joinSources(c.CSPDefaultSrc) + "; "
	}
	if len(c.CSPScriptSrc) > 0 {
		csp += "script-src " + joinSources(c.CSPScriptSrc) + "; "
	}
	if len(c.CSPStyleSrc) > 0 {
		csp += "style-src " + joinSources(c.CSPStyleSrc) + "; "
	}
	if len(c.CSPImgSrc) > 0 {
		csp += "img-src " + joinSources(c.CSPImgSrc) + "; "
	}
	if len(c.CSPConnectSrc) > 0 {
		csp += "connect-src " + joinSources(c.CSPConnectSrc) + "; "
	}
	if len(c.CSPFontSrc) > 0 {
		csp += "font-src " + joinSources(c.CSPFontSrc) + "; "
	}
	if len(c.CSPObjectSrc) > 0 {
		csp += "object-src " + joinSources(c.CSPObjectSrc) + "; "
	}
	if len(c.CSPMediaSrc) > 0 {
		csp += "media-src " + joinSources(c.CSPMediaSrc) + "; "
	}
	if len(c.CSPFrameSrc) > 0 {
		csp += "frame-src " + joinSources(c.CSPFrameSrc) + "; "
	}

	return csp
}

// joinSources joins CSP sources with spaces
func joinSources(sources []string) string {
	if len(sources) == 0 {
		return ""
	}
	result := sources[0]
	for _, source := range sources[1:] {
		result += " " + source
	}
	return result
}

// Secure returns a middleware that adds security headers
func Secure(config *SecureConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultSecureConfig()
	}

	return func(c *gin.Context) {
		// Content Security Policy
		if config.CSPEnabled {
			c.Header("Content-Security-Policy", config.buildCSPHeader())
		}

		// HTTP Strict Transport Security
		if config.HSTSEnabled {
			value := "max-age=" + strconv.Itoa(config.HSTSMaxAge)
			if config.HSTSIncludeSubdomains {
				value += "; includeSubDomains"
			}
			if config.HSTSPreload {
				value += "; preload"
			}
			c.Header("Strict-Transport-Security", value)
		}

		// X-Frame-Options
		if config.FrameGuardEnabled {
			c.Header("X-Frame-Options", config.FrameGuardAction)
		}

		// X-Content-Type-Options
		if config.ContentTypeNosniff {
			c.Header("X-Content-Type-Options", "nosniff")
		}

		// X-XSS-Protection
		if config.XSSProtection {
			c.Header("X-XSS-Protection", config.XSSProtectionMode)
		}

		// Referrer-Policy
		if config.ReferrerPolicy != "" {
			c.Header("Referrer-Policy", config.ReferrerPolicy)
		}

		c.Next()
	}
}
