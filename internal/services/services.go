// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	// Import all services to register their init functions

	_ "github.com/autobrr/dashbrr/internal/services/autobrr"
	_ "github.com/autobrr/dashbrr/internal/services/maintainerr"
	_ "github.com/autobrr/dashbrr/internal/services/omegabrr"
	_ "github.com/autobrr/dashbrr/internal/services/overseerr"
	_ "github.com/autobrr/dashbrr/internal/services/plex"
	_ "github.com/autobrr/dashbrr/internal/services/prowlarr"
	_ "github.com/autobrr/dashbrr/internal/services/radarr"
	_ "github.com/autobrr/dashbrr/internal/services/sonarr"
	_ "github.com/autobrr/dashbrr/internal/services/tailscale"
)
