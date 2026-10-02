import assert from "node:assert/strict";
import test from "node:test";

import { serviceLink } from "../src/utils/serviceLink.ts";

test("returns null for empty, scheme-less, and non-http URLs", () => {
  assert.equal(serviceLink({ url: "" }), null);
  assert.equal(serviceLink({ url: "   " }), null);
  assert.equal(serviceLink({ url: undefined }), null);
  assert.equal(serviceLink({ url: "autobrr:74747" }, "releases"), null);
  assert.equal(serviceLink({ url: "ftp://host" }), null);
  assert.equal(serviceLink({ url: "javascript:alert(1)" }), null);
  assert.equal(serviceLink({ url: "not a URL" }, "releases", { a: "b" }), null);
});

test("prefers the access URL over the URL", () => {
  assert.equal(
    serviceLink(
      { url: "http://autobrr:7474", accessUrl: "https://autobrr.example" },
      "releases"
    ),
    "https://autobrr.example/releases"
  );
  assert.equal(
    serviceLink({ url: "http://autobrr:7474", accessUrl: "" }, "releases"),
    "http://autobrr:7474/releases"
  );
});

test("an invalid access URL does not fall back to the URL", () => {
  assert.equal(
    serviceLink({ url: "http://autobrr:7474", accessUrl: "autobrr:74747" }),
    null
  );
});

test("keeps the base path and joins the path", () => {
  assert.equal(
    serviceLink({ url: "https://host/autobrr" }, "releases"),
    "https://host/autobrr/releases"
  );
  assert.equal(
    serviceLink({ url: "https://host/autobrr/" }, "/releases"),
    "https://host/autobrr/releases"
  );
});

test("adds query parameters", () => {
  assert.equal(
    serviceLink({ url: "https://host/autobrr" }, "releases", {
      action_status: "PUSH_APPROVED",
    }),
    "https://host/autobrr/releases?action_status=PUSH_APPROVED"
  );
});

test("keeps query and fragment written in the path", () => {
  assert.equal(
    serviceLink({ url: "http://nzbget:6789" }, "/?tab=config#S_SECURITY"),
    "http://nzbget:6789/?tab=config#S_SECURITY"
  );
  assert.equal(
    serviceLink({ url: "http://jellyfin:8096/" }, "/web/index.html#!/apikeys.html"),
    "http://jellyfin:8096/web/index.html#!/apikeys.html"
  );
});

test("drops query and fragment from the base URL", () => {
  assert.equal(
    serviceLink(
      { url: "https://kuma.example/internal/status/?view=all#summary" },
      `dashboard/${encodeURIComponent("42/a")}`
    ),
    "https://kuma.example/internal/status/dashboard/42%2Fa"
  );
});

test("without a path, returns the base URL", () => {
  assert.equal(
    serviceLink({ url: " https://gateway/service?token=abc#overview " }),
    "https://gateway/service?token=abc#overview"
  );
  assert.equal(serviceLink({ url: "http://autobrr:7474" }), "http://autobrr:7474/");
});
