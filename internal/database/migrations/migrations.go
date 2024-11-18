// Copyright (c) 2024, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package migrations

import "embed"

var (
	//go:embed *.sql
	SchemaMigrations embed.FS
)
