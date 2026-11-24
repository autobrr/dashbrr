import assert from "node:assert/strict";
import test from "node:test";

import { serviceFormFor, serviceForms } from "../src/config/serviceForms.ts";

test("an instance type without its own form gets the default form", () => {
  assert.equal(serviceFormFor("unknown"), serviceForms.other);
  assert.equal(serviceFormFor("constructor"), serviceForms.other);
});

test("a known type gets its own form", () => {
  assert.equal(serviceFormFor("nzbget").apiKeyLabel, "Control Password");
});

test("API key help links into the service under its base path", () => {
  const help = serviceForms.emby.apiKeyHelp({ url: "http://media:8096/emby" });
  assert.equal(help.link, "http://media:8096/emby/web/index.html#!/apikeys");
});
