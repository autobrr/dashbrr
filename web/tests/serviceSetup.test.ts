import assert from "node:assert/strict";
import test from "node:test";

import {
  addServiceSetup,
  editServiceSetup,
  serviceTestNeeded,
  serviceTestTarget,
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
  test: boolean;
}

const apiKeyService: Rules = {
  fields: ["displayName", "url", "accessUrl", "apiKey"],
  addRequired: ["displayName", "url", "apiKey"],
  editRequired: ["displayName", "url"],
  addUrl: "",
  test: true,
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
  other: { ...apiKeyService, test: false },
  traefik: optionalKeyService,
  general: { ...optionalKeyService, test: false },
  plex: {
    fields: ["displayName", "url", "accessUrl", "plexSignIn"],
    addRequired: ["displayName", "url", "plexSignIn"],
    editRequired: ["displayName", "url"],
    addUrl: "",
    test: true,
  },
  tailscale: {
    fields: ["displayName", "accessUrl", "apiKey"],
    addRequired: ["displayName", "apiKey"],
    editRequired: ["displayName"],
    addUrl: "https://api.tailscale.com",
    test: true,
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
    assert.equal(setup.mode, "add");
    assert.equal(setup.form, serviceForms[type]);
    assert.deepEqual(setup.fields, want.fields);
    assert.deepEqual(setup.required, want.addRequired);
    assert.equal(setup.values.url, want.addUrl);
  });

  test(`${type}: ${want.test ? "a new service is tested" : "no test"} before saving`, () => {
    const setup = addServiceSetup(type, "Name", {});
    const input = { displayName: "Name", url: "http://svc.test", apiKey: "key" };
    assert.equal(serviceTestNeeded(setup, input), want.test);
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

test("a new service is tested by its type, a stored one by its instance ID", () => {
  const stored = { "sonarr-1": config() };
  assert.equal(serviceTestTarget(addServiceSetup("sonarr", "Sonarr", stored)), "sonarr");
  assert.equal(serviceTestTarget(editServiceSetup("sonarr-1", stored)), "sonarr-1");
});

test("a new service takes the service name, numbered when a service has that name", () => {
  const named = (...names: string[]) =>
    Object.fromEntries(names.map((displayName, i) => [`sonarr-${i + 1}`, config({ displayName })]));
  const cases: Array<{ names: string[]; want: string }> = [
    { names: [], want: "Sonarr" },
    { names: ["Sonarr 4K"], want: "Sonarr" },
    { names: ["Sonarr"], want: "Sonarr 2" },
    { names: ["Sonarr", "Sonarr 2"], want: "Sonarr 3" },
    { names: ["Sonarr", "Sonarr 3"], want: "Sonarr 2" },
  ];
  for (const { names, want } of cases) {
    assert.equal(addServiceSetup("sonarr", "Sonarr", named(...names)).values.displayName, want, names.join(","));
  }
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

test("an edit tests the service only when its URL or API key changes", () => {
  const setup = editServiceSetup("sonarr-1", { "sonarr-1": config() });
  const cases: Array<{ change: Partial<SetupValues>; test: boolean }> = [
    { change: {}, test: false },
    { change: { displayName: "Renamed" }, test: false },
    { change: { accessUrl: "https://sonarr.example" }, test: false },
    { change: { url: "http://other.test:8989" }, test: true },
    { change: { apiKey: "new-key" }, test: true },
  ];
  for (const { change, test: want } of cases) {
    const result = setupInput(setup, { ...setup.values, ...change });
    assert.ok("input" in result);
    assert.equal(serviceTestNeeded(setup, result.input), want, JSON.stringify(change));
  }
});

test("editing the Plex token of a discovered Plex tests it", () => {
  const id = "plex-k8s-media.plex";
  const setup = editServiceSetup(id, { [id]: config({ edits: "apiKey" }) });
  const result = setupInput(setup, { ...setup.values, apiKey: "new-token" });
  assert.ok("input" in result);
  assert.equal(serviceTestNeeded(setup, result.input), true);
});
