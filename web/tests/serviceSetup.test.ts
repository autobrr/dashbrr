import assert from "node:assert/strict";
import test from "node:test";

import {
  addServiceSetup,
  editServiceSetup,
  freeInstanceId,
  setupInput,
  type SetupField,
  type SetupValues
} from "../src/config/serviceSetup.ts";
import { serviceForms } from "../src/config/serviceForms.ts";
import type { ServiceConfig, ServiceType } from "../src/types/service.ts";

interface Rules {
  fields: SetupField[];
  addRequired: SetupField[];
  editRequired: SetupField[];
  addUrl: string;
}

const apiKeyService: Rules = {
  fields: ["displayName", "url", "accessUrl", "apiKey"],
  addRequired: ["displayName", "url", "apiKey"],
  editRequired: ["displayName", "url"],
  addUrl: "",
};

const optionalKeyService: Rules = {
  ...apiKeyService,
  addRequired: ["displayName", "url"],
};

const rules: Record<ServiceType, Rules> = {
  autobrr: apiKeyService,
  radarr: apiKeyService,
  sonarr: apiKeyService,
  whisparr: apiKeyService,
  lidarr: apiKeyService,
  readarr: apiKeyService,
  bazarr: apiKeyService,
  sabnzbd: apiKeyService,
  nzbget: apiKeyService,
  prowlarr: apiKeyService,
  overseerr: apiKeyService,
  jellyfin: apiKeyService,
  emby: apiKeyService,
  uptimekuma: apiKeyService,
  maintainerr: apiKeyService,
  qui: apiKeyService,
  other: apiKeyService,
  traefik: optionalKeyService,
  general: optionalKeyService,
  plex: {
    fields: ["displayName", "url", "accessUrl", "plexSignIn"],
    addRequired: ["displayName", "url", "plexSignIn"],
    editRequired: ["displayName", "url"],
    addUrl: "",
  },
  tailscale: {
    fields: ["displayName", "accessUrl", "apiKey"],
    addRequired: ["displayName", "apiKey"],
    editRequired: ["displayName"],
    addUrl: "https://api.tailscale.com",
  },
  netbird: { ...apiKeyService, addUrl: "https://api.netbird.io" },
};

const config = (overrides: Partial<ServiceConfig> = {}): ServiceConfig => ({
  displayName: "Stored",
  url: "http://stored.test:8080",
  edits: "all",
  ...overrides,
});

for (const [type, want] of Object.entries(rules) as [ServiceType, Rules][]) {
  test(`${type}: the add form shows its fields, rules and defaults`, () => {
    const setup = addServiceSetup(type, "Name", {});
    assert.equal(setup.instanceId, `${type}-1`);
    assert.equal(setup.form, serviceForms[type]);
    assert.deepEqual(setup.fields, want.fields);
    assert.deepEqual(setup.required, want.addRequired);
    assert.equal(setup.values.url, want.addUrl);
  });

  test(`${type}: the edit form shows the add form's fields`, () => {
    const setup = editServiceSetup(`${type}-1`, { [`${type}-1`]: config() });
    assert.deepEqual(setup.fields, want.fields);
    assert.deepEqual(setup.required, want.editRequired);
  });
}

test("the edits level decides which fields the edit form shows", () => {
  const cases: Array<{ id: string; edits: ServiceConfig["edits"]; fields: SetupField[] }> = [
    { id: "sonarr-1", edits: "none", fields: [] },
    { id: "sonarr-k8s-media.sonarr", edits: "none", fields: [] },
    { id: "plex-k8s-media.plex", edits: "apiKey", fields: ["plexSignIn"] },
    { id: "sonarr-1", edits: "apiKey", fields: ["apiKey"] },
  ];
  for (const { id, edits, fields } of cases) {
    const setup = editServiceSetup(id, { [id]: config({ edits }) });
    assert.equal(setup.edits, edits, id);
    assert.deepEqual(setup.fields, fields, `${id} ${edits}`);
    assert.deepEqual(setup.required, [], `${id} ${edits}`);
  }
});

test("a new instance takes the lowest instance ID that no service has", () => {
  const cases: Array<{ ids: string[]; want: string }> = [
    { ids: [], want: "sonarr-1" },
    { ids: ["sonarr-1"], want: "sonarr-2" },
    { ids: ["sonarr-2"], want: "sonarr-1" },
    { ids: ["sonarr-1", "sonarr-3"], want: "sonarr-2" },
    { ids: ["sonarr-1", "sonarr-2", "radarr-3"], want: "sonarr-3" },
    { ids: ["sonarr-k8s-media.sonarr", "sonarr-1"], want: "sonarr-2" },
  ];
  for (const { ids, want } of cases) {
    const configurations = Object.fromEntries(ids.map((id) => [id, config()]));
    assert.equal(freeInstanceId("sonarr", configurations).instanceId, want, ids.join(","));
  }
});

test("a new instance's name carries its instance number after the first", () => {
  assert.equal(addServiceSetup("sonarr", "Sonarr", {}).values.displayName, "Sonarr");
  assert.equal(
    addServiceSetup("sonarr", "Sonarr", { "sonarr-1": config(), "sonarr-2": config() })
      .values.displayName,
    "Sonarr 3"
  );
  assert.equal(addServiceSetup("general", "General", {}).values.displayName, "");
});

test("the edit form starts from the stored configuration, with a blank API key", () => {
  const setup = editServiceSetup("sonarr-2", {
    "sonarr-2": config({ displayName: "Sonarr 4K", accessUrl: "https://sonarr.example", apiKey: "secret" }),
  });
  assert.deepEqual(setup.values, {
    displayName: "Sonarr 4K",
    url: "http://stored.test:8080",
    accessUrl: "https://sonarr.example",
    apiKey: "",
  });
});

test("editing Tailscale keeps the fixed API URL", () => {
  const setup = editServiceSetup("tailscale-1", {
    "tailscale-1": config({ url: "https://old.example" }),
  });
  assert.equal(setup.values.url, "https://api.tailscale.com");
});

const values = (overrides: Partial<SetupValues> = {}): SetupValues => ({
  displayName: "Sonarr",
  url: "http://sonarr.test:8989",
  accessUrl: "",
  apiKey: "key",
  ...overrides,
});

test("both forms trim spaces around the URL and the access URL", () => {
  const setups = [
    addServiceSetup("sonarr", "Sonarr", {}),
    editServiceSetup("sonarr-1", { "sonarr-1": config() }),
  ];
  for (const setup of setups) {
    const result = setupInput(
      setup,
      values({ url: " http://sonarr.test:8989/sonarr/ ", accessUrl: "\thttps://sonarr.example/ " })
    );
    assert.ok("input" in result);
    assert.equal(result.input.url, "http://sonarr.test:8989/sonarr/");
    assert.equal(result.input.accessUrl, "https://sonarr.example/");
  }
});

test("a blank access URL is left out", () => {
  const result = setupInput(addServiceSetup("sonarr", "Sonarr", {}), values({ accessUrl: "  " }));
  assert.ok("input" in result);
  assert.equal(result.input.accessUrl, undefined);
});

test("a missing required field stops the save with a message", () => {
  const cases: Array<{ type: ServiceType; values: Partial<SetupValues>; error: string }> = [
    { type: "sonarr", values: { apiKey: "" }, error: "API Key is required" },
    { type: "nzbget", values: { apiKey: " " }, error: "Control Password is required" },
    { type: "plex", values: { apiKey: "" }, error: "Authenticate with Plex first" },
    { type: "sonarr", values: { url: "" }, error: "URL is required" },
    { type: "sonarr", values: { url: "   " }, error: "URL is required" },
    { type: "general", values: { displayName: "" }, error: "Display Name is required" },
  ];
  for (const c of cases) {
    const result = setupInput(addServiceSetup(c.type, "Name", {}), values(c.values));
    assert.deepEqual(result, { error: c.error }, c.type);
  }
});

test("an optional API key may stay blank when adding", () => {
  for (const type of ["general", "traefik"] as const) {
    const result = setupInput(addServiceSetup(type, "Name", {}), values({ apiKey: "" }));
    assert.ok("input" in result, type);
  }
});

test("an edit may leave the API key blank to keep the stored one", () => {
  for (const type of ["sonarr", "plex", "tailscale"] as const) {
    const id = `${type}-1`;
    const result = setupInput(editServiceSetup(id, { [id]: config() }), values({ apiKey: "" }));
    assert.ok("input" in result, type);
    assert.equal(result.input.apiKey, "");
  }
});

test("a General service's edit sends a changed API key", () => {
  const setup = editServiceSetup("general-1", { "general-1": config() });
  assert.ok(setup.fields.includes("apiKey"));
  const result = setupInput(setup, values({ apiKey: "new-token" }));
  assert.ok("input" in result);
  assert.equal(result.input.apiKey, "new-token");
});
