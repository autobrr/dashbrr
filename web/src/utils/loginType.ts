/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export type LoginType = "builtin" | "oidc";

// loginTypeFrom reads the login type from a /api/auth/verify response body.
// It returns null when auth_type is missing or unknown.
export function loginTypeFrom(body: unknown): LoginType | null {
  const authType = (body as { auth_type?: unknown } | null)?.auth_type;
  return authType === "builtin" || authType === "oidc" ? authType : null;
}
