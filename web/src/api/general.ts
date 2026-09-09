/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { api } from "../utils/api";
import type { CustomServiceConfig, GeneralStatValue } from "../types/service";

export interface GeneralTestRequest {
  url: string;
  apiKey?: string;
  config: CustomServiceConfig;
}

export interface GeneralTestResult {
  status: string;
  version?: string;
  stats?: Record<string, GeneralStatValue>;
  error?: string;
}

export interface GeneralActionResult {
  status: number;
  body?: string;
}

// GET /api/general/{instanceId}/config - returns the redacted (secrets
// blanked) config for an already-configured general instance.
export const getGeneralConfig = (
  instanceId: string
): Promise<CustomServiceConfig> =>
  api.get<CustomServiceConfig>(`/api/general/${instanceId}/config`);

// PUT /api/general/{instanceId}/config - validates and persists the config.
// Must be called after the base service (url/apiKey/displayName) has been
// saved via the settings endpoint, since the instance needs to exist first.
export const saveGeneralConfig = (
  instanceId: string,
  config: CustomServiceConfig
): Promise<CustomServiceConfig> =>
  api.put<CustomServiceConfig>(`/api/general/${instanceId}/config`, config);

// POST /api/general/test - runs health + stats once against the given
// url/apiKey/config without persisting anything. Used by the "Test" button.
export const testGeneralService = (
  payload: GeneralTestRequest
): Promise<GeneralTestResult> =>
  api.post<GeneralTestResult>("/api/general/test", payload);

// POST /api/general/{instanceId}/actions/{actionId} - runs a configured
// action. Actions with `confirm: true` require the X-Confirm header, or the
// server responds 428 Precondition Required.
//
// Goes through the shared `api` helper (rather than a raw fetch) so this
// call gets the same session-expiry redirect, 429 retry/backoff, and
// timeout handling as every other API call in the app.
export const runGeneralAction = (
  instanceId: string,
  actionId: string,
  confirm: boolean
): Promise<GeneralActionResult> =>
  api.post<GeneralActionResult>(
    `/api/general/${instanceId}/actions/${actionId}`,
    undefined,
    undefined,
    confirm ? { "X-Confirm": "yes" } : undefined
  );
