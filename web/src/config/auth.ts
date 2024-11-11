/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

// Get the current frontend URL including base URL
const getFrontendUrl = () => {
  // In development, use localhost:3000
  if (import.meta.env.DEV) {
    return 'http://localhost:3000';
  }
  
  // In production, use the origin and configured base URL
  const origin = window.location.origin;
  const baseUrl = window.__BASE_URL__ || '';
  return baseUrl && baseUrl !== '/' ? `${origin}${baseUrl}` : origin;
};

// Common auth endpoints
const COMMON_ENDPOINTS = {
  config: 'api/auth/config',
  userInfo: 'api/auth/userinfo',
};

// OIDC-specific endpoints
const OIDC_ENDPOINTS = {
  login: `api/auth/oidc/login?frontendUrl=${encodeURIComponent(getFrontendUrl())}`,
  callback: `api/auth/oidc/callback?frontendUrl=${encodeURIComponent(getFrontendUrl())}`,
  logout: `api/auth/oidc/logout?frontendUrl=${encodeURIComponent(getFrontendUrl())}`,
  refresh: 'api/auth/oidc/refresh',
  verify: 'api/auth/oidc/verify',
  userInfo: 'api/auth/oidc/userinfo',
};

// Built-in auth endpoints
const BUILTIN_ENDPOINTS = {
  login: 'api/auth/login',
  register: 'api/auth/register',
  logout: 'api/auth/logout',
  verify: 'api/auth/verify',
  registrationStatus: 'api/auth/registration-status',
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
  default: 'builtin' | 'oidc';
}

export async function getAuthConfig(): Promise<AuthConfig> {
  try {
    const response = await fetch(buildAuthUrl(AUTH_URLS.config));
    if (!response.ok) {
      throw new Error('Failed to fetch auth configuration');
    }
    return await response.json();
  } catch (error) {
    console.error('Error fetching auth config:', error);
    // Return default configuration if fetch fails
    return {
      methods: {
        builtin: true,
        oidc: false,
      },
      default: 'builtin',
    };
  }
}

// Helper function to build URLs with base URL
export function buildAuthUrl(path: string): string {
  // Remove any leading/trailing slashes from path
  const cleanPath = path.replace(/^\/+|\/+$/g, '');

  // In development, return just the path to use Vite proxy
  if (import.meta.env.DEV) {
    return `/${cleanPath}`;
  }

  // In production, use the origin and configured base URL
  const origin = window.location.origin;
  const baseUrl = window.__BASE_URL__ || '';

  // Combine all parts ensuring no double slashes
  if (baseUrl && baseUrl !== '/') {
    return `${origin}${baseUrl}/${cleanPath}`;
  }
  return `${origin}/${cleanPath}`;
}
