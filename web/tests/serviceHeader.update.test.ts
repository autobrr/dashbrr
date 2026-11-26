import assert from "node:assert/strict";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { ServiceHeader } from "../src/components/ui/ServiceHeader.tsx";
import type { ServiceType } from "../src/types/service.ts";

const render = (displayName: string, serviceType: ServiceType) =>
  renderToStaticMarkup(
    createElement(ServiceHeader, {
      displayName,
      serviceType,
      version: "4.0.15.2941",
      updateAvailable: true,
      onConfigure: () => {},
      onRemove: () => {},
      edits: "all",
    })
  );

test("a renamed card links Update to the releases of its service type", () => {
  assert.match(
    render("Sonarr 4K", "sonarr"),
    /<a href="https:\/\/github\.com\/Sonarr\/Sonarr\/releases"[^>]*>Update<\/a>/
  );
});

test("an Uptime Kuma card links Update to the Uptime Kuma releases", () => {
  assert.match(
    render("Uptime Kuma", "uptimekuma"),
    /<a href="https:\/\/github\.com\/louislam\/uptime-kuma\/releases"[^>]*>Update<\/a>/
  );
});

test("a type without a releases page shows the version instead of Update", () => {
  const html = render("Health", "general");
  assert.doesNotMatch(html, /Update/);
  assert.match(html, />4\.0\.15</);
});
