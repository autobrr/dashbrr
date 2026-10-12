import assert from "node:assert/strict";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { ServiceHeader } from "../src/components/ui/ServiceHeader.tsx";
import type { ServiceType } from "../src/types/service.ts";

const render = (serviceType: ServiceType, version: string) =>
  renderToStaticMarkup(
    createElement(ServiceHeader, {
      displayName: "Media",
      serviceType,
      version,
      onConfigure: () => {},
      onRemove: () => {},
    })
  );

test("an Emby card shows all four parts of the version", () => {
  assert.match(render("emby", "4.10.0.40"), />4\.10\.0\.40</);
});

test("other cards show three parts of the version", () => {
  const html = render("sonarr", "4.0.15.2941");
  assert.match(html, />4\.0\.15</);
  assert.doesNotMatch(html, /2941/);
});
