/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { NetBirdPeer, Service, ServiceType, TailscaleDevice } from "../types/service";

// A member is one row of a mesh network list. Tailscale calls its members
// "devices", NetBird calls them "peers".
export type NetworkMember = {
  id: string;
  name: string;
  online: boolean;
  updateAvailable: boolean;
  expired?: boolean;
  lastSeen: string;
  ip: string;
  version: string;
  chips?: { label: string; values: string[] };
  details: { label: string; value?: string }[];
  searchFields: string[];
};

export interface NetworkDescriptor {
  type: "tailscale" | "netbird";
  label: string;
  instancePrefix: string;
  noun: { singular: string; plural: string };
  searchPlaceholder: string;
  updateLink: string;
  members: (service?: Service) => NetworkMember[];
  reportedCounts: (service?: Service) => { total?: number; online?: number } | undefined;
  connectedText: (counts: { total: number; online: number }) => string;
  serverUpdateAvailable: (service?: Service) => boolean;
}

export const toTailscaleMember = (device: TailscaleDevice): NetworkMember => ({
  id: device.id,
  name: device.name,
  online: device.online,
  updateAvailable: device.updateAvailable,
  lastSeen: device.lastSeen,
  ip: device.ipAddress,
  version: device.clientVersion,
  chips: device.tags && device.tags.length > 0 ? { label: "Tags", values: device.tags } : undefined,
  details: [{ label: "Type", value: device.deviceType }],
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
  lastSeen: peer.lastSeen,
  ip: peer.ip,
  version: peer.version,
  chips: peer.groups && peer.groups.length > 0 ? { label: "Groups", values: peer.groups } : undefined,
  details: [
    { label: "OS", value: peer.os },
    {
      label: "Location",
      value: [peer.cityName, peer.countryCode].filter(Boolean).join(", ") || undefined,
    },
  ],
  searchFields: [
    peer.name,
    peer.os,
    peer.ip,
    ...(peer.groups ?? []),
    peer.cityName,
  ].filter(Boolean),
});

export const NETWORK_DESCRIPTORS: Record<NetworkDescriptor["type"], NetworkDescriptor> = {
  tailscale: {
    type: "tailscale",
    label: "Tailscale",
    instancePrefix: "tailscale-",
    noun: { singular: "device", plural: "devices" },
    searchPlaceholder: "Search devices by name, type, IP, or tags...",
    updateLink: "https://tailscale.com/changelog#client",
    members: (service) => (service?.stats?.tailscale?.devices ?? []).map(toTailscaleMember),
    reportedCounts: (service) => service?.details?.tailscale,
    connectedText: () => "Connected",
    serverUpdateAvailable: () => false,
  },
  netbird: {
    type: "netbird",
    label: "NetBird",
    instancePrefix: "netbird-",
    noun: { singular: "peer", plural: "peers" },
    searchPlaceholder: "Search peers by name, OS, IP, group, or city...",
    updateLink: "https://github.com/netbirdio/netbird/releases",
    members: (service) => (service?.stats?.netbird?.peers ?? []).map(toNetBirdMember),
    reportedCounts: (service) => service?.details?.netbird,
    connectedText: ({ total, online }) => `${online}/${total} peers online`,
    serverUpdateAvailable: (service) => Boolean(service?.details?.netbird?.updateAvailable),
  },
};

// The order of the header buttons, the same order the Add Services menu uses.
export const NETWORK_ORDER: NetworkDescriptor["type"][] = ["tailscale", "netbird"];

// Mesh networks live in the page header with one instance each, so the service
// grid and the Add Services menu treat them apart.
export const isMeshNetworkType = (type: ServiceType): boolean =>
  (NETWORK_ORDER as ServiceType[]).includes(type);

// findInstanceID returns the first instance of a network, or null when the user
// has not set that network up.
export function findInstanceID(
  configurations: Record<string, unknown>,
  prefix: string
): string | null {
  const found = Object.keys(configurations)
    .sort()
    .find((id) => id.startsWith(prefix));

  return found ?? null;
}

export function filterMembers<T extends NetworkMember>(members: T[], searchTerm: string): T[] {
  const needle = searchTerm.trim().toLowerCase();
  if (!needle) {
    return members;
  }

  return members.filter((member) =>
    member.searchFields.some((field) => field?.toLowerCase().includes(needle))
  );
}

// sortMembers puts the members that need work first: an update, then online
// members, then by name.
export function sortMembers<T extends NetworkMember>(members: T[]): T[] {
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

export function prepareMembers<T extends NetworkMember>(members: T[], searchTerm: string): T[] {
  return sortMembers(filterMembers(members, searchTerm));
}

// memberCounts returns total and online from one snapshot: the loaded list wins,
// and the reported counts only fill the gap until the list has arrived.
export function memberCounts(
  members: NetworkMember[],
  reported?: { total?: number; online?: number }
): { total: number; online: number } {
  if (members.length > 0) {
    return {
      total: members.length,
      online: members.filter((member) => member.online).length,
    };
  }

  return { total: reported?.total ?? 0, online: reported?.online ?? 0 };
}

export type NetworkStatusKind =
  | "loading"
  | "error"
  | "pending"
  | "warning"
  | "offline"
  | "connected";

// networkStatus decides what the header button says. The backend decides when a
// network is a warning, for example a NetBird token that sees no peers.
export function networkStatus(input: { status?: string; online: number }): NetworkStatusKind {
  switch (input.status) {
    case undefined:
    case "loading":
      return "loading";
    case "error":
    case "pending":
    case "warning":
      return input.status;
  }

  return input.online === 0 ? "offline" : "connected";
}
