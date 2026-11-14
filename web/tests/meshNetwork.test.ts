import assert from "node:assert/strict";
import test from "node:test";

import {
  filterMembers,
  findInstanceID,
  networkStatus,
  prepareMembers,
  sortMembers,
  toNetBirdMember,
  toTailscaleMember,
  NETWORK_DESCRIPTORS,
  NETWORK_ORDER,
  type NetworkMember
} from "../src/utils/meshNetwork.ts";
import type { NetBirdPeer, TailscaleDevice } from "../src/types/service.ts";

const member = (overrides: Partial<NetworkMember>): NetworkMember => ({
  id: "id",
  name: "name",
  online: false,
  updateAvailable: false,
  searchFields: [],
  ...overrides,
});

test("update badge comes before online before name", () => {
  const sorted = sortMembers([
    member({ id: "1", name: "yin", online: true }),
    member({ id: "2", name: "alpha", online: false }),
    member({ id: "3", name: "beta", online: false, updateAvailable: true }),
    member({ id: "4", name: "zeta", online: true }),
  ]);

  assert.deepEqual(
    sorted.map((entry) => entry.id),
    ["3", "1", "4", "2"]
  );
});

test("sortMembers does not mutate its input", () => {
  const input = [member({ id: "1", name: "zeta" }), member({ id: "2", name: "alpha" })];
  sortMembers(input);

  assert.deepEqual(input.map((entry) => entry.id), ["1", "2"]);
});

test("netbird search covers name, os, ip, group and city", () => {
  const peer: NetBirdPeer = {
    id: "peer-1",
    name: "host-1",
    ip: "100.111.0.10",
    online: true,
    lastSeen: "2026-11-14T10:05:26Z",
    os: "Linux",
    version: "0.60.0",
    updateAvailable: false,
    groups: ["homelab"],
    cityName: "Kiel",
    countryCode: "DE",
    loginExpired: false,
  };
  const members = [toNetBirdMember(peer)];

  for (const needle of ["host", "linux", "100.111", "homelab", "kiel"]) {
    assert.equal(filterMembers(members, needle).length, 1, `search for ${needle}`);
  }
  assert.equal(filterMembers(members, "berlin").length, 0);
});

test("tailscale search stays on name, type, IP and tags", () => {
  const device: TailscaleDevice = {
    id: "device-1",
    name: "laptop",
    ipAddress: "100.64.0.7",
    lastSeen: "2026-11-14T10:05:26Z",
    online: true,
    deviceType: "macOS",
    clientVersion: "1.80.0",
    updateAvailable: true,
    tags: ["tag:work"],
  };
  const members = [toTailscaleMember(device)];

  assert.equal(filterMembers(members, "macos").length, 1);
  assert.equal(filterMembers(members, "tag:work").length, 1);
  assert.equal(filterMembers(members, "kiel").length, 0);
});

test("an empty search term keeps every member", () => {
  const members = [member({ id: "1" }), member({ id: "2" })];

  assert.equal(filterMembers(members, "   ").length, 2);
});

test("prepareMembers filters and then sorts", () => {
  const prepared = prepareMembers(
    [
      member({ id: "1", name: "host-1", online: true, searchFields: ["homelab"] }),
      member({ id: "2", name: "host-2", online: false, searchFields: ["office"] }),
    ],
    "office"
  );

  assert.deepEqual(prepared.map((entry) => entry.id), ["2"]);
});

test("findInstanceID takes the first instance of the network", () => {
  const configurations = {
    "netbird-2": {},
    "tailscale-1": {},
    "netbird-1": {},
  };

  assert.equal(findInstanceID(configurations, "tailscale-"), "tailscale-1");
  assert.equal(findInstanceID(configurations, "netbird-"), "netbird-1");
  assert.equal(findInstanceID(configurations, "wireguard-"), null);
});

test("the header shows Tailscale before NetBird", () => {
  assert.deepEqual(NETWORK_ORDER, ["tailscale", "netbird"]);
  assert.equal(NETWORK_DESCRIPTORS.tailscale.noun.plural, "devices");
  assert.equal(NETWORK_DESCRIPTORS.netbird.noun.plural, "peers");
});

test("networkStatus separates empty from offline and error", () => {
  assert.equal(networkStatus({ status: undefined, total: 0, online: 0 }), "loading");
  assert.equal(networkStatus({ status: "loading", total: 0, online: 0 }), "loading");
  assert.equal(networkStatus({ status: "error", total: 0, online: 0 }), "error");
  assert.equal(networkStatus({ status: "pending", total: 0, online: 0 }), "pending");
  assert.equal(networkStatus({ status: "warning", total: 0, online: 0 }), "empty");
  assert.equal(networkStatus({ status: "online", total: 3, online: 0 }), "offline");
  assert.equal(networkStatus({ status: "online", total: 3, online: 2 }), "connected");
});
