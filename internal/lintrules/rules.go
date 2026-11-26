//go:build ruleguard

// Copyright (c) 2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package lintrules

import "github.com/quasilyte/go-ruleguard/dsl"

func serviceURLByHand(m dsl.Matcher) {
	m.Match(`fmt.Sprintf($format, $*_)`).
		Where(m["format"].Text.Matches(`^"%s(/|\?|%s)`) &&
			m.File().PkgPath.Matches(`/internal/services/`) &&
			!m.File().Name.Matches(`_test\.go$`)).
		Report("build a service request URL with core.RequestURL or core.RequestURLQuery, not with fmt.Sprintf")

	m.Match(`$base + $path`).
		Where(m["path"].Const &&
			m["path"].Text.Matches(`^"[/?]`) &&
			!m["base"].Const &&
			m.File().PkgPath.Matches(`/internal/services/`) &&
			!m.File().Name.Matches(`_test\.go$`)).
		Report("build a service request URL with core.RequestURL or core.RequestURLQuery, not with +")

	m.Match(`url.Parse($base + $_)`, `neturl.Parse($base + $_)`).
		Where(m.File().PkgPath.Matches(`/internal/services/`) &&
			!m.File().Name.Matches(`_test\.go$`)).
		Report("build a service request URL with core.RequestURL or core.RequestURLQuery, not with url.Parse of a joined string")

	m.Match(`$u.Path = $_ + $_`).
		Where(m["u"].Type.Is(`*url.URL`) &&
			m.File().PkgPath.Matches(`/internal/services/`) &&
			!m.File().Name.Matches(`_test\.go$`)).
		Report("build a service request URL with core.RequestURL or core.RequestURLQuery, not by joining URL.Path by hand")
}
