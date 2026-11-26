/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import assert from "node:assert/strict";
import test from "node:test";
import {
  getBazarrIssueProviders,
  getBazarrProvidersWithIssuesCount,
  getBazarrSignalRStatusColor,
  isBazarrProviderIssue
} from "../src/components/services/bazarr/status.ts";

test("filters healthy Good providers from issue list and count", () => {
  const providers = [
    { name: "provider-a", status: "Good", retry: "-" },
    { name: "provider-b", status: " good ", retry: "-" },
  ];

  assert.equal(isBazarrProviderIssue(providers[0]), false);
  assert.deepEqual(getBazarrIssueProviders(providers), []);
  assert.equal(getBazarrProvidersWithIssuesCount(providers, 0), 0);
  assert.equal(getBazarrProvidersWithIssuesCount(providers, 1), 1);

  const mixed = [
    ...providers,
    { name: "provider-c", status: "Throttled", retry: "10m" },
  ];
  assert.deepEqual(getBazarrIssueProviders(mixed), [mixed[2]]);
  assert.equal(getBazarrProvidersWithIssuesCount(mixed, 0), 1);
});

test("maps SignalR states to semantic colors", () => {
  assert.equal(getBazarrSignalRStatusColor("LIVE"), "text-green-400");
  assert.equal(getBazarrSignalRStatusColor("DOWN"), "text-red-400");
  assert.equal(getBazarrSignalRStatusColor("down"), "text-red-400");
  assert.equal(getBazarrSignalRStatusColor("CONNECTING"), "text-amber-400");
});
