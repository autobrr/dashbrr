import assert from "node:assert/strict";
import test from "node:test";
import {
  deriveHealthUpdate,
  hydrateServicesFromConfigurations,
  mergeServicePatchSnapshot,
  type ServicePatchSnapshot,
} from "../src/hooks/serviceData/merge.ts";
import type { Service } from "../src/types/service.ts";

const config = {
  "radarr-1": {
    url: "https://radarr.example",
    displayName: "Radarr",
    apiKey: "key",
  },
};

test("hydrate_configurations keeps runtime status on refresh", () => {
  const previous = new Map<string, Service>([
    [
      "radarr-1",
      {
        id: "radarr-1",
        instanceId: "radarr-1",
        name: "Radarr",
        displayName: "Radarr",
        type: "radarr",
        status: "online",
        message: "Healthy",
        url: "https://radarr.example",
        apiKey: "key",
        stats: { radarr: { queue: { totalRecords: 1 } } },
      },
    ],
  ]);

  const next = hydrateServicesFromConfigurations(previous, config, new Map());
  const hydrated = next.get("radarr-1");

  assert.ok(hydrated);
  assert.equal(hydrated.status, "online");
  assert.equal(hydrated.message, "Healthy");
  assert.deepEqual(hydrated.stats, { radarr: { queue: { totalRecords: 1 } } });
});

test("hydrate_configurations does not seed configured services with placeholder message", () => {
  const hydrated = hydrateServicesFromConfigurations(new Map(), config, new Map()).get(
    "radarr-1"
  );

  assert.ok(hydrated);
  assert.equal(hydrated.status, "loading");
  assert.equal(hydrated.message, undefined);
});

test("hydrate_configurations applies cached internal warning snapshot", () => {
  const snapshot: ServicePatchSnapshot = mergeServicePatchSnapshot(
    undefined,
    {
      stats: { radarr: { queue: { totalRecords: 2 } } },
      details: { radarr: { queueCount: 2 } },
      message: "radarr_queue",
    },
    "warning"
  );

  const hydrated = hydrateServicesFromConfigurations(
    new Map(),
    config,
    new Map([["radarr-1", snapshot]])
  ).get("radarr-1");

  assert.ok(hydrated);
  assert.equal(hydrated.status, "warning");
  assert.equal(hydrated.message, "radarr_queue");
  assert.deepEqual(hydrated.stats, { radarr: { queue: { totalRecords: 2 } } });
  assert.deepEqual(hydrated.details, { radarr: { queueCount: 2 } });
});

test("hydrate_configurations promotes loading to online from internal snapshots", () => {
  const snapshot: ServicePatchSnapshot = mergeServicePatchSnapshot(
    undefined,
    {
      stats: { radarr: { queue: { totalRecords: 3 } } },
      details: { radarr: { queueCount: 3 } },
      message: "radarr_queue",
    },
    "online"
  );

  const hydrated = hydrateServicesFromConfigurations(
    new Map(),
    config,
    new Map([["radarr-1", snapshot]])
  ).get("radarr-1");

  assert.ok(hydrated);
  assert.equal(hydrated.status, "online");
  assert.equal(hydrated.message, "radarr_queue");
  assert.deepEqual(hydrated.stats, { radarr: { queue: { totalRecords: 3 } } });
});

test("deriveHealthUpdate applies health state even when message is empty", () => {
  const update = deriveHealthUpdate({
    serviceId: "general-1",
    status: "online",
    message: "",
    eventType: "health",
    responseTime: 4,
    version: "1.0.0",
    lastChecked: "2026-02-19T00:00:00Z",
  });

  assert.ok(update);
  assert.equal(update.instanceId, "general-1");
  assert.equal(update.patch.status, "online");
  assert.equal(update.patch.message, "");
  assert.equal(update.patch.responseTime, 4);
  assert.equal(update.patch.version, "1.0.0");
});

test("hydrate_configurations keeps warning/version/responseTime after internal update", () => {
  const healthUpdate = deriveHealthUpdate({
    serviceId: "prowlarr-1",
    status: "warning",
    message: "[IndexerLongTermStatusCheck] Indexers unavailable",
    eventType: "health",
    responseTime: 6,
    version: "2.3.2",
    lastChecked: "2026-02-19T00:00:00Z",
  });

  assert.ok(healthUpdate);

  const internalUpdate = deriveHealthUpdate({
    serviceId: "prowlarr-1",
    status: "online",
    message: "prowlarr_indexers",
    eventType: "internal",
    stats: {
      prowlarr: {
        indexers: [{ id: 1, name: "One" }],
      },
    },
    lastChecked: "2026-02-19T00:00:01Z",
  });

  assert.ok(internalUpdate);

  let snapshot: ServicePatchSnapshot = mergeServicePatchSnapshot(
    undefined,
    healthUpdate.patch,
    healthUpdate.internalStatus
  );
  snapshot = mergeServicePatchSnapshot(
    snapshot,
    internalUpdate.patch,
    internalUpdate.internalStatus
  );

  const hydrated = hydrateServicesFromConfigurations(
    new Map(),
    {
      "prowlarr-1": {
        url: "https://prowlarr.example",
        displayName: "Prowlarr",
        apiKey: "key",
      },
    },
    new Map([["prowlarr-1", snapshot]])
  ).get("prowlarr-1");

  assert.ok(hydrated);
  assert.equal(hydrated.status, "warning");
  assert.equal(
    hydrated.message,
    "[IndexerLongTermStatusCheck] Indexers unavailable"
  );
  assert.equal(hydrated.version, "2.3.2");
  assert.equal(hydrated.responseTime, 6);
  assert.deepEqual(hydrated.stats, {
    prowlarr: {
      indexers: [{ id: 1, name: "One" }],
    },
  });
});

test("hydrate_configurations keeps update flags when internal payload includes reset-like fields", () => {
  const healthUpdate = deriveHealthUpdate({
    serviceId: "radarr-1",
    status: "online",
    message: "Healthy",
    eventType: "health",
    responseTime: 42,
    updateAvailable: true,
    version: "6.1.1",
    lastChecked: "2026-02-19T00:00:00Z",
  });
  assert.ok(healthUpdate);

  const internalUpdate = deriveHealthUpdate({
    serviceId: "radarr-1",
    status: "online",
    message: "radarr_queue",
    eventType: "internal",
    responseTime: 0,
    updateAvailable: false,
    stats: {
      radarr: {
        queue: { totalRecords: 2 },
      },
    },
    lastChecked: "2026-02-19T00:00:01Z",
  });
  assert.ok(internalUpdate);

  let snapshot: ServicePatchSnapshot = mergeServicePatchSnapshot(
    undefined,
    healthUpdate.patch,
    healthUpdate.internalStatus
  );
  snapshot = mergeServicePatchSnapshot(
    snapshot,
    internalUpdate.patch,
    internalUpdate.internalStatus
  );

  const hydrated = hydrateServicesFromConfigurations(
    new Map(),
    config,
    new Map([["radarr-1", snapshot]])
  ).get("radarr-1");

  assert.ok(hydrated);
  assert.equal(hydrated.status, "online");
  assert.equal(hydrated.message, "Healthy");
  assert.equal(hydrated.responseTime, 42);
  assert.equal(hydrated.updateAvailable, true);
  assert.equal(hydrated.version, "6.1.1");
  assert.deepEqual(hydrated.stats, {
    radarr: {
      queue: { totalRecords: 2 },
    },
  });
});

test("mergeServicePatchSnapshot deep-merges nested stats payloads", () => {
  let snapshot: ServicePatchSnapshot = mergeServicePatchSnapshot(
    undefined,
    {
      stats: {
        prowlarr: {
          stats: { grabCount: 1, failCount: 2, indexerCount: 3 },
          indexers: [{ id: 10, name: "Alpha" }],
        },
      },
    },
    "online"
  );

  snapshot = mergeServicePatchSnapshot(
    snapshot,
    {
      stats: {
        prowlarr: {
          stats: { grabCount: 9 },
        },
      },
    },
    "online"
  );

  const hydrated = hydrateServicesFromConfigurations(
    new Map(),
    {
      "prowlarr-1": {
        url: "https://prowlarr.example",
        displayName: "Prowlarr",
        apiKey: "key",
      },
    },
    new Map([["prowlarr-1", snapshot]])
  ).get("prowlarr-1");

  assert.ok(hydrated);
  assert.deepEqual(hydrated.stats, {
    prowlarr: {
      stats: { grabCount: 9, failCount: 2, indexerCount: 3 },
      indexers: [{ id: 10, name: "Alpha" }],
    },
  });
});
