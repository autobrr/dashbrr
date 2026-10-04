import assert from "node:assert/strict";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { ServiceHeader } from "../src/components/ui/ServiceHeader.tsx";

const render = (discovered: boolean) =>
  renderToStaticMarkup(
    createElement(ServiceHeader, {
      displayName: "Radarr",
      url: "http://radarr:7878",
      onConfigure: () => {},
      onRemove: () => {},
      discovered,
    })
  );

test("a discovered service shows the Kubernetes icon and no edit or delete controls", () => {
  const html = render(true);
  assert.match(html, /title="Managed by Kubernetes discovery"/);
  assert.doesNotMatch(html, /Configure service/);
  assert.doesNotMatch(html, /Remove service/);
});

test("a service added in the UI keeps its edit and delete controls", () => {
  const html = render(false);
  assert.doesNotMatch(html, /Managed by Kubernetes discovery/);
  assert.match(html, /Configure service/);
  assert.match(html, /Remove service/);
});
