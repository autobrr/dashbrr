import assert from "node:assert/strict";
import test from "node:test";
import {
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

test("hydrate_configurations applies cached internal status snapshot", () => {
  const snapshot: ServicePatchSnapshot = mergeServicePatchSnapshot(
    undefined,
    {
      stats: { radarr: { queue: { totalRecords: 2 } } },
      details: { radarr: { queueCount: 2 } },
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
  assert.deepEqual(hydrated.stats, { radarr: { queue: { totalRecords: 2 } } });
  assert.deepEqual(hydrated.details, { radarr: { queueCount: 2 } });
});
