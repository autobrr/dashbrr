/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import {
  Service,
  ServiceConfig,
  ServiceHealth,
  ServiceStatus,
  ServiceType,
} from "../../types/service";
import { serviceTemplates } from "../../config/serviceTemplates";

const templateByType: Map<string, (typeof serviceTemplates)[number]> = new Map(
  serviceTemplates.map((template) => [template.type, template] as const)
);

type HealthPatchPresence = {
  hasVersion: boolean;
  hasUpdateAvailable: boolean;
  hasResponseTime: boolean;
};

const INTERNAL_EVENT_PATTERN = /^[a-z0-9]+(?:_[a-z0-9]+)+$/;

const hasOwnProperty = (value: unknown, key: string): boolean =>
  typeof value === "object" &&
  value !== null &&
  Object.prototype.hasOwnProperty.call(value, key);

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const parseDate = (value: unknown): Date | undefined => {
  if (!value) return undefined;
  if (value instanceof Date) return value;
  if (typeof value === "string" || typeof value === "number") {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? undefined : date;
  }
  return undefined;
};

const isInternalEventMessage = (message: string): boolean =>
  INTERNAL_EVENT_PATTERN.test(message.trim());

const isInternalServiceEvent = (health: ServiceHealth): boolean =>
  health.eventType === "internal"
    ? true
    : health.eventType === "health"
      ? false
      : isInternalEventMessage(health.message || "");

export const mergeServicePayload = <T extends object>(
  current: T | undefined,
  incoming: T | undefined
): T | undefined => {
  if (!incoming) return current;
  if (!current) return { ...incoming };

  const merged = { ...current } as T;
  for (const key of Object.keys(incoming) as Array<keyof T>) {
    const nextValue = incoming[key];
    const prevValue = current[key];

    if (isRecord(prevValue) && isRecord(nextValue)) {
      (merged as Record<string, unknown>)[key as string] = {
        ...prevValue,
        ...nextValue,
      };
      continue;
    }

    merged[key] = nextValue;
  }

  return merged;
};

const buildServicePatchFromHealth = (
  health: ServiceHealth,
  presence: HealthPatchPresence
): Partial<Service> => {
  const shouldApplyHealthState =
    health.message !== "" && !isInternalServiceEvent(health);

  const patch: Partial<Service> = {
    lastChecked: health.lastChecked,
    stats: health.stats,
    details: health.details,
  };

  if (shouldApplyHealthState) {
    patch.status = health.status;
    patch.message = health.message;
    patch.health = health;
  }

  if (shouldApplyHealthState && presence.hasUpdateAvailable) {
    patch.updateAvailable = Boolean(health.updateAvailable);
  }

  if (shouldApplyHealthState && presence.hasResponseTime) {
    patch.responseTime = health.responseTime;
  }

  if (presence.hasVersion) {
    patch.version = health.version;
  }

  return patch;
};

const buildServiceFromConfig = (
  instanceId: string,
  config: ServiceConfig
): Service => {
  const [type] = instanceId.split("-");
  const template = templateByType.get(type);
  const hasRequiredConfig = Boolean(config.url);

  return {
    id: instanceId,
    instanceId,
    name: template?.name || "Unknown Service",
    type: (template?.type || "other") as ServiceType,
    status: (hasRequiredConfig ? "loading" : "pending") as ServiceStatus,
    url: config.url,
    accessUrl: config.accessUrl,
    apiKey: config.apiKey,
    displayName: config.displayName,
    healthEndpoint: template?.healthEndpoint,
    message: hasRequiredConfig ? "Waiting for updates" : "Service not configured",
    stats: {},
    details: {},
  };
};

export const mergeServiceWithPatch = (
  current: Service,
  patch: Partial<Service>
): Service => ({
  ...current,
  ...patch,
  stats: mergeServicePayload(current.stats, patch.stats),
  details: mergeServicePayload(current.details, patch.details),
});

export const applyServicePatch = (
  services: Map<string, Service>,
  instanceId: string,
  patch: Partial<Service>
): Map<string, Service> => {
  const current = services.get(instanceId);
  if (!current) return services;

  const next = new Map(services);
  next.set(instanceId, mergeServiceWithPatch(current, patch));
  return next;
};

export const deriveHealthUpdate = (
  payload: unknown
):
  | {
      instanceId: string;
      patch: Partial<Service>;
    }
  | undefined => {
  if (typeof payload !== "object" || payload === null) {
    return undefined;
  }

  const raw = payload as ServiceHealth;
  const instanceId = raw.serviceId;
  if (!instanceId) {
    return undefined;
  }

  const lastChecked = parseDate(
    (raw as unknown as { lastChecked?: unknown }).lastChecked
  );
  const health: ServiceHealth = {
    ...raw,
    lastChecked: lastChecked || new Date(),
  };

  const patch = buildServicePatchFromHealth(health, {
    hasVersion: hasOwnProperty(payload, "version"),
    hasUpdateAvailable: hasOwnProperty(payload, "updateAvailable"),
    hasResponseTime: hasOwnProperty(payload, "responseTime"),
  });

  return {
    instanceId,
    patch,
  };
};

export const hydrateServicesFromConfigurations = (
  previous: Map<string, Service>,
  configurations: Record<string, ServiceConfig>,
  latestPatchByInstance: Map<string, Partial<Service>>
): Map<string, Service> => {
  const next = new Map(previous);
  const configuredIds = new Set(Object.keys(configurations));

  for (const [instanceId, config] of Object.entries(configurations)) {
    const base = buildServiceFromConfig(instanceId, config);
    const existing = next.get(instanceId);
    let merged = existing ? { ...existing, ...base } : base;

    const patch = latestPatchByInstance.get(instanceId);
    if (patch) {
      merged = mergeServiceWithPatch(merged, patch);
    }

    next.set(instanceId, merged);
  }

  for (const instanceId of next.keys()) {
    if (!configuredIds.has(instanceId)) {
      next.delete(instanceId);
    }
  }

  return next;
};
