/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

// Client-side mirror of internal/models/custom.go's CustomServiceConfig.Validate().
// This is a best-effort structural check so the "Import JSON" button and the
// form itself can reject an obviously bad definition before it ever reaches
// the server; the server remains the source of truth for validation.

import type {
  CustomActionConfig,
  CustomHealthConfig,
  CustomLoginConfig,
  CustomServiceConfig,
  CustomStatConfig
} from "../../../types/service";

export interface CustomServiceConfigValidation {
  ok: boolean;
  config?: CustomServiceConfig;
  errors: string[];
}

const MAX_ENTRIES = 8;
const AUTH_MODES = new Set(["none", "header", "query", "basic", "bearer"]);
const HEALTH_LOGIN_METHODS = new Set(["", "GET", "POST"]);
const INJECT_AS_VALUES = new Set(["", "header", "query", "cookie", "bearer"]);
const STAT_FORMATS = new Set(["", "number", "bytes", "duration", "percent", "text"]);
const ACTION_METHODS = new Set(["", "GET", "POST", "PUT", "DELETE"]);
const ACTION_ID_PATTERN = /^[a-z0-9-]+$/;

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const isOptionalString = (value: unknown): value is string | undefined =>
  value === undefined || typeof value === "string";

const isStringArray = (value: unknown): value is string[] =>
  value === undefined ||
  (Array.isArray(value) && value.every((item) => typeof item === "string"));

const validateAuth = (value: unknown, errors: string[]): void => {
  if (value === undefined) return;
  if (!isRecord(value)) {
    errors.push("auth must be an object");
    return;
  }
  if (typeof value.mode !== "string" || value.mode === "") {
    errors.push("auth.mode is required when auth is present");
  } else if (!AUTH_MODES.has(value.mode)) {
    errors.push(`auth.mode "${value.mode}" is invalid`);
  }
  for (const key of ["headerName", "queryParam", "username", "password", "token"]) {
    if (!isOptionalString(value[key])) {
      errors.push(`auth.${key} must be a string`);
    }
  }
};

const validateLogin = (value: unknown, errors: string[]): void => {
  if (value === undefined) return;
  if (!isRecord(value)) {
    errors.push("login must be an object");
    return;
  }
  const login = value as Partial<CustomLoginConfig>;
  if (login.method !== undefined && !HEALTH_LOGIN_METHODS.has(login.method)) {
    errors.push(`login.method "${String(login.method)}" is invalid`);
  }
  if (login.injectAs !== undefined && !INJECT_AS_VALUES.has(login.injectAs)) {
    errors.push(`login.injectAs "${String(login.injectAs)}" is invalid`);
  }
  for (const key of [
    "path",
    "contentType",
    "body",
    "captureCookie",
    "captureJSONPath",
    "injectName",
  ]) {
    if (!isOptionalString(value[key])) {
      errors.push(`login.${key} must be a string`);
    }
  }
};

const validateHealth = (value: unknown, errors: string[]): void => {
  if (value === undefined) return;
  if (!isRecord(value)) {
    errors.push("health must be an object");
    return;
  }
  const health = value as Partial<CustomHealthConfig>;
  if (typeof health.path !== "string" || health.path === "") {
    errors.push("health.path is required when health is present");
  }
  if (health.method !== undefined && !HEALTH_LOGIN_METHODS.has(health.method)) {
    errors.push(`health.method "${String(health.method)}" is invalid`);
  }
  if (!isOptionalString(value.body)) errors.push("health.body must be a string");
  if (!isOptionalString(value.statusPath)) errors.push("health.statusPath must be a string");
  if (!isOptionalString(value.versionPath)) errors.push("health.versionPath must be a string");
  if (!isStringArray(value.okValues)) errors.push("health.okValues must be an array of strings");
  if (!isStringArray(value.warnValues)) errors.push("health.warnValues must be an array of strings");
};

const validateStats = (value: unknown, errors: string[]): void => {
  if (value === undefined) return;
  if (!Array.isArray(value)) {
    errors.push("stats must be an array");
    return;
  }
  if (value.length > MAX_ENTRIES) {
    errors.push(`stats: at most ${MAX_ENTRIES} entries allowed, got ${value.length}`);
  }
  value.forEach((item: unknown, index: number) => {
    if (!isRecord(item)) {
      errors.push(`stats[${index}] must be an object`);
      return;
    }
    const stat = item as Partial<CustomStatConfig>;
    if (typeof stat.label !== "string" || stat.label === "") {
      errors.push(`stats[${index}].label is required`);
    }
    if (typeof stat.path !== "string" || stat.path === "") {
      errors.push(`stats[${index}].path is required`);
    }
    if (stat.format !== undefined && !STAT_FORMATS.has(stat.format)) {
      errors.push(`stats[${index}].format "${String(stat.format)}" is invalid`);
    }
    if (!isOptionalString(item.unit)) {
      errors.push(`stats[${index}].unit must be a string`);
    }
  });
};

const validateActions = (value: unknown, errors: string[]): void => {
  if (value === undefined) return;
  if (!Array.isArray(value)) {
    errors.push("actions must be an array");
    return;
  }
  if (value.length > MAX_ENTRIES) {
    errors.push(`actions: at most ${MAX_ENTRIES} entries allowed, got ${value.length}`);
  }
  value.forEach((item: unknown, index: number) => {
    if (!isRecord(item)) {
      errors.push(`actions[${index}] must be an object`);
      return;
    }
    const action = item as Partial<CustomActionConfig>;
    if (typeof action.id !== "string" || action.id === "") {
      errors.push(`actions[${index}].id is required`);
    } else if (!ACTION_ID_PATTERN.test(action.id)) {
      errors.push(`actions[${index}].id "${action.id}" is invalid: must match ^[a-z0-9-]+$`);
    }
    if (typeof action.label !== "string" || action.label === "") {
      errors.push(`actions[${index}].label is required`);
    }
    if (typeof action.path !== "string" || action.path === "") {
      errors.push(`actions[${index}].path is required`);
    }
    if (action.method !== undefined && !ACTION_METHODS.has(action.method)) {
      errors.push(`actions[${index}].method "${String(action.method)}" is invalid`);
    }
    if (!isOptionalString(item.body)) {
      errors.push(`actions[${index}].body must be a string`);
    }
    if (item.confirm !== undefined && typeof item.confirm !== "boolean") {
      errors.push(`actions[${index}].confirm must be a boolean`);
    }
  });
};

// Validates a parsed value against the CustomServiceConfig shape. Does not
// mutate the input; returns the same reference as `config` on success.
export const validateCustomServiceConfig = (
  value: unknown
): CustomServiceConfigValidation => {
  const errors: string[] = [];

  if (!isRecord(value)) {
    return { ok: false, errors: ["Definition must be a JSON object"] };
  }

  validateAuth(value.auth, errors);
  validateLogin(value.login, errors);
  validateHealth(value.health, errors);
  validateStats(value.stats, errors);
  validateActions(value.actions, errors);

  if (
    value.timeoutSeconds !== undefined &&
    (typeof value.timeoutSeconds !== "number" || Number.isNaN(value.timeoutSeconds))
  ) {
    errors.push("timeoutSeconds must be a number");
  }

  if (errors.length > 0) {
    return { ok: false, errors };
  }

  return { ok: true, config: value as CustomServiceConfig, errors: [] };
};

// Returns a copy of config with credential fields blanked out, safe to copy
// to the clipboard or display in the (unencrypted, easy-to-mis-share)
// import textarea. Mirrors the fields models.CustomServiceConfig.Redacted()
// blanks on the backend: auth.password, auth.token, and login.body (the
// login body commonly embeds the raw {{username}}/{{password}} template
// values via substitution, and can carry other secrets too).
export const redactForExport = (
  config: CustomServiceConfig
): CustomServiceConfig => {
  const redacted: CustomServiceConfig = { ...config };

  if (redacted.auth) {
    redacted.auth = { ...redacted.auth, password: "", token: "" };
  }
  if (redacted.login) {
    redacted.login = { ...redacted.login, body: "" };
  }

  return redacted;
};

// Parses and validates a JSON string (as pasted into the "Import JSON" box).
export const parseCustomServiceConfigJSON = (
  input: string
): CustomServiceConfigValidation => {
  let parsed: unknown;
  try {
    parsed = JSON.parse(input);
  } catch (err) {
    return {
      ok: false,
      errors: [`Invalid JSON: ${err instanceof Error ? err.message : String(err)}`],
    };
  }
  return validateCustomServiceConfig(parsed);
};
