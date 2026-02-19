import assert from "node:assert/strict";
import test from "node:test";

import { hasMeaningfulServiceContent } from "../src/utils/serviceCardContent.ts";
import type { Service } from "../src/types/service.ts";

const makeService = (overrides: Partial<Service>): Service => ({
  id: "svc-1",
  instanceId: "svc-1",
  name: "Service",
  displayName: "Service",
  type: "general",
  status: "online",
  url: "https://service.local",
  ...overrides,
});

test("plex without streams compacts body", () => {
  const service = makeService({
    type: "plex",
    details: { plex: { activeStreams: 0, transcoding: 0 } },
    stats: { plex: { sessions: [] } },
  });

  assert.equal(hasMeaningfulServiceContent(service), false);
});

test("plex with active streams keeps full spacing", () => {
  const service = makeService({
    type: "plex",
    details: { plex: { activeStreams: 1, transcoding: 0 } },
  });

  assert.equal(hasMeaningfulServiceContent(service), true);
});

test("arr queue with no records compacts body", () => {
  const service = makeService({
    type: "radarr",
    stats: { radarr: { queue: { totalRecords: 0, records: [] } } },
  });

  assert.equal(hasMeaningfulServiceContent(service), false);
});

test("actionable warning message always keeps full spacing", () => {
  const service = makeService({
    type: "radarr",
    status: "warning",
    message: "Indexer unavailable",
    stats: { radarr: { queue: { totalRecords: 0, records: [] } } },
  });

  assert.equal(hasMeaningfulServiceContent(service), true);
});

test("autobrr always keeps full spacing for stat tiles", () => {
  const service = makeService({ type: "autobrr" });

  assert.equal(hasMeaningfulServiceContent(service), true);
});
