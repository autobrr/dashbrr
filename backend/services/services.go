// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package services

import (
	// Import all services to register their init functions

	_ "github.com/autobrr/dashbrr/backend/services/autobrr"
	_ "github.com/autobrr/dashbrr/backend/services/maintainerr"
	_ "github.com/autobrr/dashbrr/backend/services/omegabrr"
	_ "github.com/autobrr/dashbrr/backend/services/overseerr"
	_ "github.com/autobrr/dashbrr/backend/services/plex"
	_ "github.com/autobrr/dashbrr/backend/services/prowlarr"
	_ "github.com/autobrr/dashbrr/backend/services/radarr"
	_ "github.com/autobrr/dashbrr/backend/services/sonarr"
	_ "github.com/autobrr/dashbrr/backend/services/tailscale"
)
