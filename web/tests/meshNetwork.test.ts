import assert from "node:assert/strict";
import test from "node:test";

import {
  filterMembers,
  memberCounts,
  findInstanceID,
  isMeshNetworkType,
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
  lastSeen: "",
  ip: "",
  version: "",
  details: [],
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

test("networkStatus takes the warning from the backend, not from an empty list", () => {
  assert.equal(networkStatus({ status: undefined, online: 0 }), "loading");
  assert.equal(networkStatus({ status: "loading", online: 0 }), "loading");
  assert.equal(networkStatus({ status: "error", online: 0 }), "error");
  assert.equal(networkStatus({ status: "pending", online: 0 }), "pending");
  assert.equal(networkStatus({ status: "warning", online: 0 }), "warning");
  // A Tailscale account with no devices is online with zero devices: Offline, as on develop.
  assert.equal(networkStatus({ status: "online", online: 0 }), "offline");
  assert.equal(networkStatus({ status: "online", online: 2 }), "connected");
});

test("Tailscale says Connected and NetBird counts its peers", () => {
  const counts = { total: 18, online: 12 };

  assert.equal(NETWORK_DESCRIPTORS.tailscale.connectedText(counts), "Connected");
  assert.equal(NETWORK_DESCRIPTORS.netbird.connectedText(counts), "12/18 peers online");
});

test("only NetBird reports a server update on the button", () => {
  const service = {
    updateAvailable: true,
    details: { netbird: { total: 1, online: 1, updateAvailable: true } },
  } as unknown as Parameters<typeof NETWORK_DESCRIPTORS.netbird.serverUpdateAvailable>[0];

  assert.equal(NETWORK_DESCRIPTORS.netbird.serverUpdateAvailable(service), true);
  assert.equal(NETWORK_DESCRIPTORS.tailscale.serverUpdateAvailable(service), false);
});

test("tailscale tags and netbird groups become chips", () => {
  const tags = toTailscaleMember({
    id: "d", name: "laptop", ipAddress: "100.64.0.7", lastSeen: "", online: true,
    deviceType: "macOS", clientVersion: "", updateAvailable: false, tags: ["tag:work"],
  });
  assert.deepEqual(tags.chips, { label: "Tags", values: ["tag:work"] });

  const untagged = toTailscaleMember({
    id: "d", name: "laptop", ipAddress: "100.64.0.7", lastSeen: "", online: true,
    deviceType: "macOS", clientVersion: "", updateAvailable: false, tags: [],
  });
  assert.equal(untagged.chips, undefined);
});

test("the grid and the menu treat only Tailscale and NetBird as mesh networks", () => {
  assert.equal(isMeshNetworkType("tailscale"), true);
  assert.equal(isMeshNetworkType("netbird"), true);
  assert.equal(isMeshNetworkType("sonarr"), false);
});

test("memberCounts keeps total and online from one snapshot", () => {
  const members = [member({ id: "a", online: true }), member({ id: "b", online: false })];

  // The loaded list wins even when a newer health check reports more online peers.
  assert.deepEqual(memberCounts(members, { total: 2, online: 3 }), { total: 2, online: 1 });
});

test("memberCounts falls back to the reported counts before the list arrives", () => {
  assert.deepEqual(memberCounts([], { total: 4, online: 2 }), { total: 4, online: 2 });
  assert.deepEqual(memberCounts([], undefined), { total: 0, online: 0 });
});
