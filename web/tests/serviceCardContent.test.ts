import assert from "node:assert/strict";
import test from "node:test";

import {
  getServiceCardLayoutClasses,
  SERVICE_CARD_LAYOUT,
  hasMeaningfulServiceContent
} from "../src/utils/serviceCardContent.ts";
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

test("general service with no stats/actions/legacy details compacts body", () => {
  const service = makeService({ type: "general" });

  assert.equal(hasMeaningfulServiceContent(service), false);
});

// Regression test: a general service using the legacy no-Health-section
// probe only ever populates details.general (key/value fields from the
// response body), never stats.general.stats/actions - it was previously
// misclassified as having no meaningful content.
test("general service with only legacy details.general keeps full spacing", () => {
  const service = makeService({
    type: "general",
    details: { general: { uptime: "3d", version: "1.2.3" } },
  });

  assert.equal(hasMeaningfulServiceContent(service), true);
});

test("general service with only configured stats keeps full spacing", () => {
  const service = makeService({
    type: "general",
    stats: { general: { stats: { Uptime: { display: "3d" } } } },
  });

  assert.equal(hasMeaningfulServiceContent(service), true);
});

test("general service with only configured actions keeps full spacing", () => {
  const service = makeService({
    type: "general",
    stats: { general: { actions: [{ id: "restart", label: "Restart" }] } },
  });

  assert.equal(hasMeaningfulServiceContent(service), true);
});

test("service card layout snapshot stays stable", () => {
  assert.deepEqual(SERVICE_CARD_LAYOUT, {
    compact: {
      bodyMarginClass: "mt-1",
      footerSpacingClass: "mt-2 pt-2",
    },
    regular: {
      bodyMarginClass: "mt-2",
      footerSpacingClass: "mt-4 pt-4",
    },
  });
});

test("status-only cards use compact spacing classes", () => {
  assert.deepEqual(getServiceCardLayoutClasses("compact"), {
    bodyMarginClass: "mt-1",
    footerSpacingClass: "mt-2 pt-2",
  });
});
