/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

// Common auth endpoints
const COMMON_ENDPOINTS = {
  config: "/api/auth/config",
  verify: "/api/auth/verify",
};

// OIDC-specific endpoints
const OIDC_ENDPOINTS = {
  login: "/api/auth/oidc/login",
  logout: "/api/auth/oidc/logout",
};

// Built-in auth endpoints
const BUILTIN_ENDPOINTS = {
  login: "/api/auth/login",
  register: "/api/auth/register",
  logout: "/api/auth/logout",
};

export const AUTH_URLS = {
  ...COMMON_ENDPOINTS,
  oidc: OIDC_ENDPOINTS,
  builtin: BUILTIN_ENDPOINTS,
};

export interface AuthConfig {
  methods: {
    builtin: boolean;
    oidc: boolean;
  };
  default: "builtin" | "oidc";
}

export async function getAuthConfig(): Promise<AuthConfig> {
  try {
    const response = await fetch(AUTH_URLS.config);
    if (!response.ok) {
      throw new Error("Failed to fetch auth configuration");
    }
    return await response.json();
  } catch (error) {
    console.error("Error fetching auth config:", error);
    // Return default configuration if fetch fails
    return {
      methods: {
        builtin: true,
        oidc: false,
      },
      default: "builtin",
    };
  }
}
