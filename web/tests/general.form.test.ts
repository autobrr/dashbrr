import assert from "node:assert/strict";
import test from "node:test";

import {
  parseCustomServiceConfigJSON,
  validateCustomServiceConfig
} from "../src/components/configuration/general/customServiceConfig.ts";

const validDefinition = {
  auth: { mode: "bearer", token: "secret" },
  login: {
    method: "POST",
    path: "/login",
    injectAs: "header",
    injectName: "Authorization",
  },
  health: {
    method: "GET",
    path: "/health",
    statusPath: "status",
    okValues: ["ok"],
    warnValues: ["degraded"],
    versionPath: "version",
  },
  stats: [
    { label: "Active Users", path: "data.activeUsers", unit: "users", format: "number" },
  ],
  actions: [{ id: "restart-service", label: "Restart", path: "/restart", confirm: true }],
  timeoutSeconds: 15,
};

test("Import JSON accepts a valid custom service definition", () => {
  const result = parseCustomServiceConfigJSON(JSON.stringify(validDefinition));

  assert.equal(result.ok, true);
  assert.deepEqual(result.errors, []);
  assert.equal(result.config?.auth?.mode, "bearer");
  assert.equal(result.config?.stats?.[0]?.label, "Active Users");
});

test("Import JSON rejects malformed JSON text", () => {
  const result = parseCustomServiceConfigJSON("{not valid json");

  assert.equal(result.ok, false);
  assert.ok(result.errors[0].startsWith("Invalid JSON"));
});

test("Import JSON rejects a definition with an invalid auth mode", () => {
  const result = validateCustomServiceConfig({ auth: { mode: "carrier-pigeon" } });

  assert.equal(result.ok, false);
  assert.ok(result.errors.some((error) => error.includes("auth.mode")));
});

test("Import JSON rejects more than 8 stats", () => {
  const stats = Array.from({ length: 9 }, (_, i) => ({
    label: `Stat ${i}`,
    path: `data.stat${i}`,
  }));

  const result = validateCustomServiceConfig({ stats });

  assert.equal(result.ok, false);
  assert.ok(result.errors.some((error) => error.includes("at most 8 entries")));
});

test("Import JSON rejects an action with a bad id", () => {
  const result = validateCustomServiceConfig({
    actions: [{ id: "Not Valid!", label: "Bad", path: "/x" }],
  });

  assert.equal(result.ok, false);
  assert.ok(result.errors.some((error) => error.includes("actions[0].id")));
});

test("Import JSON rejects a health block missing a path", () => {
  const result = validateCustomServiceConfig({ health: { method: "GET" } });

  assert.equal(result.ok, false);
  assert.ok(result.errors.some((error) => error.includes("health.path")));
});

test("Import JSON rejects a non-object definition", () => {
  const result = validateCustomServiceConfig(["not", "an", "object"]);

  assert.equal(result.ok, false);
  assert.deepEqual(result.errors, ["Definition must be a JSON object"]);
});

test("an empty definition is valid (all fields optional)", () => {
  const result = validateCustomServiceConfig({});

  assert.equal(result.ok, true);
  assert.deepEqual(result.errors, []);
});
