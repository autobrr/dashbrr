/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { NetBirdPeer, TailscaleDevice } from "../types/service";

// A network is one mesh VPN that the header shows. Tailscale calls its members
// "devices", NetBird calls them "peers", so the words travel with the network.
export type NetworkMember = {
  id: string;
  name: string;
  online: boolean;
  updateAvailable: boolean;
  searchFields: string[];
  expired?: boolean;
};

export interface NetworkDescriptor {
  type: "tailscale" | "netbird";
  label: string;
  instancePrefix: string;
  noun: { singular: string; plural: string };
}

export const NETWORK_DESCRIPTORS: Record<NetworkDescriptor["type"], NetworkDescriptor> = {
  tailscale: {
    type: "tailscale",
    label: "Tailscale",
    instancePrefix: "tailscale-",
    noun: { singular: "device", plural: "devices" },
  },
  netbird: {
    type: "netbird",
    label: "NetBird",
    instancePrefix: "netbird-",
    noun: { singular: "peer", plural: "peers" },
  },
};

// The order of the header buttons. A user who runs both networks sees Tailscale
// first, the same order the Add Services menu uses.
export const NETWORK_ORDER: NetworkDescriptor["type"][] = ["tailscale", "netbird"];

export const toTailscaleMember = (device: TailscaleDevice): NetworkMember => ({
  id: device.id,
  name: device.name,
  online: device.online,
  updateAvailable: device.updateAvailable,
  searchFields: [
    device.name,
    device.deviceType,
    device.ipAddress,
    ...(device.tags ?? []),
  ].filter(Boolean),
});

export const toNetBirdMember = (peer: NetBirdPeer): NetworkMember => ({
  id: peer.id,
  name: peer.name,
  online: peer.online,
  updateAvailable: peer.updateAvailable,
  expired: peer.loginExpired,
  searchFields: [
    peer.name,
    peer.os,
    peer.ip,
    ...(peer.groups ?? []),
    peer.cityName,
  ].filter(Boolean),
});

// findInstanceID returns the first instance of a network, or null when the user
// has not set that network up. Each network shows only its first instance.
export function findInstanceID(
  configurations: Record<string, unknown>,
  prefix: string
): string | null {
  const found = Object.keys(configurations)
    .sort()
    .find((id) => id.startsWith(prefix));

  return found ?? null;
}

// filterMembers narrows the list by one search term over the fields of the network.
export function filterMembers(
  members: NetworkMember[],
  searchTerm: string
): NetworkMember[] {
  const needle = searchTerm.trim().toLowerCase();
  if (!needle) {
    return members;
  }

  return members.filter((member) =>
    member.searchFields.some((field) => field?.toLowerCase().includes(needle))
  );
}

// sortMembers puts the members that need work first: an update badge, then online
// members, then by name. This is the order of the Tailscale device list.
export function sortMembers(members: NetworkMember[]): NetworkMember[] {
  return [...members].sort((a, b) => {
    if (a.updateAvailable !== b.updateAvailable) {
      return b.updateAvailable ? 1 : -1;
    }
    if (a.online !== b.online) {
      return b.online ? 1 : -1;
    }

    return a.name.localeCompare(b.name);
  });
}

export function prepareMembers(
  members: NetworkMember[],
  searchTerm: string
): NetworkMember[] {
  return sortMembers(filterMembers(members, searchTerm));
}

// countOnline returns how many members are connected. It uses the count that the
// backend reported when there is one, so the button and the list agree.
export function countOnline(
  members: NetworkMember[],
  reportedOnline?: number
): number {
  return reportedOnline ?? members.filter((member) => member.online).length;
}

export type NetworkStatusKind =
  | "loading"
  | "error"
  | "pending"
  | "empty"
  | "offline"
  | "connected";

// networkStatus decides what the header button says. The order matters: an error
// hides an empty list, and an empty list is a warning, not an offline network.
export function networkStatus(input: {
  status?: string;
  total: number;
  online: number;
}): NetworkStatusKind {
  if (input.status === "loading" || input.status === undefined) {
    return "loading";
  }
  if (input.status === "error") {
    return "error";
  }
  if (input.status === "pending") {
    return "pending";
  }
  if (input.total === 0) {
    return "empty";
  }
  if (input.online === 0) {
    return "offline";
  }

  return "connected";
}
