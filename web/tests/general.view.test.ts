import assert from "node:assert/strict";
import test from "node:test";

import { buildGeneralStatsView } from "../src/components/services/general/generalStatsView.ts";
import type { Service } from "../src/types/service.ts";

const makeService = (overrides: Partial<Service>): Service => ({
  id: "general-1",
  instanceId: "general-1",
  name: "General Service",
  displayName: "My API",
  type: "general",
  status: "online",
  url: "https://api.example.com",
  ...overrides,
});

test("builds stat tiles, sorted by label, from stats.general.stats", () => {
  const service = makeService({
    stats: {
      general: {
        stats: {
          "Active Users": { display: "42", raw: 42, unit: "users" },
          "Queue Depth": { display: "3", raw: 3 },
        },
        actions: [{ id: "restart", label: "Restart", confirm: true }],
      },
    },
  });

  const view = buildGeneralStatsView(service);

  assert.deepEqual(
    view.tiles.map((tile) => tile.label),
    ["Active Users", "Queue Depth"]
  );
  assert.deepEqual(
    view.tiles.map((tile) => tile.display),
    ["42", "3"]
  );
  assert.equal(view.tiles[0].unit, "users");
});

test("exposes actions from stats.general.actions, including confirm flag", () => {
  const service = makeService({
    stats: {
      general: {
        actions: [
          { id: "restart", label: "Restart", confirm: true },
          { id: "sync", label: "Sync now" },
        ],
      },
    },
  });

  const view = buildGeneralStatsView(service);

  assert.equal(view.actions.length, 2);
  assert.ok(view.actions.some((action) => action.id === "restart" && action.confirm === true));
  assert.ok(view.actions.some((action) => action.id === "sync" && !action.confirm));
});

test("falls back to service.health.stats.general when service.stats.general is absent", () => {
  const service = makeService({
    health: {
      status: "online",
      message: "general_stats",
      serviceId: "general-1",
      stats: {
        general: {
          stats: { Uptime: { display: "3d" } },
          actions: [{ id: "sync", label: "Sync now" }],
        },
      },
    },
  });

  const view = buildGeneralStatsView(service);

  assert.deepEqual(
    view.tiles.map((tile) => tile.label),
    ["Uptime"]
  );
  assert.equal(view.actions.length, 1);
});

test("no tiles/actions when neither stats.general nor health.stats.general is set", () => {
  const service = makeService({});
  const view = buildGeneralStatsView(service);

  assert.deepEqual(view.tiles, []);
  assert.deepEqual(view.actions, []);
});

test("reads the legacy key/value list from the top-level details.general", () => {
  const service = makeService({
    details: { general: { uptime: "3d", region: "us-east" } },
  });

  const view = buildGeneralStatsView(service);

  assert.deepEqual(
    view.detailFields.map((field) => field.key),
    ["region", "uptime"]
  );
});

test("detail fields and stats.general are independent (both can be present)", () => {
  const service = makeService({
    details: { general: { uptime: "3d" } },
    stats: {
      general: {
        stats: { Queue: { display: "3" } },
      },
    },
  });

  const view = buildGeneralStatsView(service);

  assert.deepEqual(view.detailFields, [{ key: "uptime", label: "Uptime", value: "3d" }]);
  assert.deepEqual(
    view.tiles.map((tile) => tile.label),
    ["Queue"]
  );
});

test("shows the message panel when status is not online, even without text", () => {
  const service = makeService({ status: "warning", message: "" });
  const view = buildGeneralStatsView(service);

  assert.equal(view.showMessage, true);
});
