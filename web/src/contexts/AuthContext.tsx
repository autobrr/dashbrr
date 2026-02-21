/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { createContext, useState, useEffect, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import {
  AuthContextType,
  User,
  LoginCredentials,
  RegisterCredentials,
} from "../types/auth";
import { AUTH_URLS, getAuthConfig, AuthConfig } from "../config/auth";
import { readErrorMessage } from "../utils/http";

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Utility function for exponential backoff
const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
const MAX_RETRIES = 5;

const fetchWith429Retry = async (
  url: string,
  init: RequestInit,
  maxRetries = MAX_RETRIES
): Promise<Response> => {
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    const response = await fetch(url, init);
    if (response.status !== 429 || attempt === maxRetries - 1) {
      return response;
    }

    const retryAfter = response.headers.get("Retry-After");
    const parsedRetryAfter = retryAfter ? Number.parseInt(retryAfter, 10) : NaN;
    const waitTime =
      Number.isFinite(parsedRetryAfter) && parsedRetryAfter >= 0
        ? parsedRetryAfter * 1000
        : Math.min(1000 * Math.pow(2, attempt), 30000);
    await wait(waitTime);
  }

  throw new Error("request retry loop exhausted");
};

const debug = (...args: unknown[]) => {
  if (import.meta.env.DEV) {
    // Keep auth chatter out of production logs.
    console.debug(...args);
  }
};

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null);
  const [authType, setAuthType] = useState<"oidc" | "builtin" | null>(null);

  const clearAuth = useCallback(() => {
    debug("[AuthProvider] Clearing authentication state");
    setUser(null);
    setIsAuthenticated(false);
    setAuthType(null);
  }, []);

  const checkAuthStatus = useCallback(async () => {
    debug("[AuthProvider] Checking auth status");
    setLoading(true);

    const candidates: Array<"oidc" | "builtin"> = ["oidc", "builtin"];

    const baseRequest: RequestInit = {
      credentials: "include",
    };

    const verify = async (url: string): Promise<boolean> => {
      const response = await fetchWith429Retry(url, baseRequest);
      return response.ok;
    };

    try {
      let detected: "oidc" | "builtin" | null = null;
      for (const t of candidates) {
        const verifyUrl = t === "oidc" ? AUTH_URLS.oidc.verify : AUTH_URLS.builtin.verify;
        if (await verify(verifyUrl)) {
          detected = t;
          break;
        }
      }

      if (!detected) {
        clearAuth();
        return;
      }

      setAuthType(detected);

      const userInfoUrl = detected === "oidc" ? AUTH_URLS.oidc.userInfo : AUTH_URLS.userInfo;
      const userInfoResponse = await fetchWith429Retry(userInfoUrl, baseRequest);
      if (!userInfoResponse.ok) {
        clearAuth();
        return;
      }

      const userData = await userInfoResponse.json();
      setUser({ ...userData, auth_type: detected });
      setIsAuthenticated(true);
    } catch (error) {
      console.error("[AuthProvider] Auth check failed:", error);
      clearAuth();
    } finally {
      setLoading(false);
    }
  }, [clearAuth]);

  useEffect(() => {
    let mounted = true;
    debug("[AuthProvider] Initializing auth provider");

    const init = async () => {
      const config = await getAuthConfig();
      if (!mounted) return;

      debug("[AuthProvider] Received auth config:", config);
      setAuthConfig(config);

      if (config.bypass) {
        debug("[AuthProvider] Auth bypass enabled");
        setAuthType("builtin");
        setUser({
          id: 1,
          username: "auth-bypass",
          email: "bypass@dashbrr.local",
          auth_type: "builtin",
        });
        setIsAuthenticated(true);
        setLoading(false);
        return;
      }

      // Always check once: supports cookie-only sessions.
      await checkAuthStatus();
    };

    void init();

    return () => {
      mounted = false;
    };
  }, [checkAuthStatus]);

  const submitAuthForm = async (
    url: string,
    payload: unknown,
    fallbackMessage: string
  ): Promise<Response> => {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
      credentials: "include",
    });

    if (!response.ok) {
      const message = await readErrorMessage(response);
      throw new Error(message || fallbackMessage);
    }

    return response;
  };

  const loginWithOIDC = () => {
    debug("[AuthProvider] Initiating OIDC login");
    if (!authConfig?.methods.oidc) {
      throw new Error("OIDC authentication is not configured");
    }
    clearAuth();
    window.location.href = AUTH_URLS.oidc.login;
  };

  const login = async (credentials?: LoginCredentials) => {
    debug(
      "[AuthProvider] Login attempt",
      credentials ? "with credentials" : "with OIDC"
    );
    if (!credentials) {
      return loginWithOIDC();
    }

    try {
      const response = await submitAuthForm(
        AUTH_URLS.builtin.login,
        credentials,
        "Login failed"
      );
      await response.json();
      debug("[AuthProvider] Login successful");
      await checkAuthStatus();
    } catch (error) {
      console.error("[AuthProvider] Login error:", error);
      throw error;
    }
  };

  const register = async (credentials: RegisterCredentials) => {
    debug("[AuthProvider] Registration attempt");
    try {
      await submitAuthForm(
        AUTH_URLS.builtin.register,
        credentials,
        "Registration failed"
      );

      debug(
        "[AuthProvider] Registration successful, proceeding to login"
      );
      // After successful registration, log in with the same credentials
      await login({
        username: credentials.username,
        password: credentials.password,
      });
    } catch (error) {
      console.error("[AuthProvider] Registration error:", error);
      throw error;
    }
  };

  const logout = async () => {
    debug("[AuthProvider] Initiating logout");
    try {
      const currentAuthType =
        authType || user?.auth_type || "builtin";
      const logoutUrl =
        currentAuthType === "oidc"
          ? AUTH_URLS.oidc.logout
          : AUTH_URLS.builtin.logout;

      debug(
        "[AuthProvider] Logging out with auth type:",
        currentAuthType
      );

      if (currentAuthType === "oidc") {
        // Must be a navigation to follow provider redirects.
        clearAuth();
        window.location.href = logoutUrl;
        return;
      }

      const response = await fetch(logoutUrl, {
        method: "POST",
        credentials: "include",
      });

      if (!response.ok) {
        console.error("[AuthProvider] Logout request failed:", response.status);
        throw new Error("Logout failed");
      }

      debug("[AuthProvider] Logout successful");
      clearAuth();
      navigate("/login", { replace: true });
    } catch (error) {
      console.error("[AuthProvider] Logout error:", error);
      clearAuth();
      navigate("/login", { replace: true });
    }
  };

  const value: AuthContextType = {
    isAuthenticated,
    user,
    login,
    loginWithOIDC,
    register,
    logout,
    loading,
    authConfig,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export { AuthContext };
