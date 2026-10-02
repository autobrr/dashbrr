import assert from "node:assert/strict";
import test from "node:test";

import {
  getUptimeKumaMonitorView,
  type UptimeKumaFilter
} from "../src/components/services/uptimekuma/uptimeKumaView.ts";
import type { UptimeKumaMonitor } from "../src/types/service.ts";

const monitors: UptimeKumaMonitor[] = [
  { id: "1", name: "API", type: "http", status: "up", responseTimeMs: 24 },
  { id: "2", name: "Database", type: "tcp", status: "down" },
  { id: "3", name: "Worker", type: "docker", status: "pending" },
  { id: "4", name: "Website", type: "http", status: "maintenance" },
];

test("default monitor view contains only monitors needing attention", () => {
  const view = getUptimeKumaMonitorView(monitors, null);

  assert.equal(view.title, "Needs Attention");
  assert.deepEqual(
    view.monitors.map((monitor) => monitor.id),
    ["2", "3"]
  );
});

test("explicit monitor filters return only their matching status", () => {
  const cases: Array<{
    filter: UptimeKumaFilter;
    title: string;
    ids: string[];
  }> = [
    { filter: "total", title: "All Monitors", ids: ["1", "2", "3", "4"] },
    { filter: "up", title: "Up Monitors", ids: ["1"] },
    { filter: "down", title: "Down Monitors", ids: ["2"] },
    { filter: "pending", title: "Pending Monitors", ids: ["3"] },
    { filter: "maintenance", title: "Maintenance Monitors", ids: ["4"] },
  ];

  for (const { filter, title, ids } of cases) {
    const view = getUptimeKumaMonitorView(monitors, filter);
    assert.equal(view.title, title);
    assert.deepEqual(
      view.monitors.map((monitor) => monitor.id),
      ids
    );
  }
});

test("default monitor view prioritizes down before pending", () => {
  const unordered: UptimeKumaMonitor[] = [
    { id: "1", name: "Alpha", status: "pending" },
    { id: "2", name: "Zulu", status: "down" },
    { id: "3", name: "Beta", status: "down" },
  ];
  const view = getUptimeKumaMonitorView(unordered, null);

  assert.deepEqual(
    view.monitors.map((monitor) => monitor.id),
    ["3", "2", "1"]
  );
});
