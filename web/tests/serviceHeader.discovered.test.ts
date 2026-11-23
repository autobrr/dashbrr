import assert from "node:assert/strict";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { ServiceHeader } from "../src/components/ui/ServiceHeader.tsx";

import type { ServiceEdits } from "../src/types/service.ts";

const render = (edits: ServiceEdits) =>
  renderToStaticMarkup(
    createElement(ServiceHeader, {
      displayName: "Radarr",
      url: "http://radarr:7878",
      onConfigure: () => {},
      onRemove: () => {},
      edits,
    })
  );

test("a discovered service shows the Kubernetes icon and no edit or delete controls", () => {
  const html = render("none");
  assert.match(html, /title="Managed by Kubernetes discovery"/);
  assert.doesNotMatch(html, /Configure service|Sign in with Plex/);
  assert.doesNotMatch(html, /Remove service/);
});

test("a discovered Plex keeps the Plex sign-in and no delete control", () => {
  const html = render("apiKey");
  assert.match(html, /title="Sign in with Plex"/);
  assert.doesNotMatch(html, /Remove service/);
});

test("a service added in the UI keeps its edit and delete controls", () => {
  const html = render("all");
  assert.doesNotMatch(html, /Managed by Kubernetes discovery/);
  assert.match(html, /Configure service/);
  assert.match(html, /Remove service/);
});
