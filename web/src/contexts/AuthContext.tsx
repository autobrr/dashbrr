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
    localStorage.removeItem("auth_type");
    setUser(null);
    setIsAuthenticated(false);
    setAuthType(null);
  }, []);

  const checkAuthStatus = useCallback(async () => {
    const MAX_RETRIES = 5;
    debug("[AuthProvider] Checking auth status");
    setLoading(true);

    const storedAuthType = localStorage.getItem("auth_type") as
      | "oidc"
      | "builtin"
      | null;

    const candidates: Array<"oidc" | "builtin"> = storedAuthType
      ? storedAuthType === "oidc"
        ? ["oidc", "builtin"]
        : ["builtin", "oidc"]
      : ["oidc", "builtin"];

    const baseRequest: RequestInit = {
      credentials: "include",
    };

    const verify = async (url: string): Promise<boolean> => {
      let attempt = 0;
      while (attempt < MAX_RETRIES) {
        const res = await fetch(url, baseRequest);
        if (res.status === 429) {
          const retryAfter = res.headers.get("Retry-After");
          const waitTime = retryAfter
            ? parseInt(retryAfter) * 1000
            : Math.min(1000 * Math.pow(2, attempt), 30000);
          await wait(waitTime);
          attempt++;
          continue;
        }
        return res.ok;
      }
      return false;
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
      localStorage.setItem("auth_type", detected);

      const userInfoUrl = detected === "oidc" ? AUTH_URLS.oidc.userInfo : AUTH_URLS.userInfo;
      let attempt = 0;
      while (attempt < MAX_RETRIES) {
        const res = await fetch(userInfoUrl, baseRequest);
        if (res.status === 429) {
          const retryAfter = res.headers.get("Retry-After");
          const waitTime = retryAfter
            ? parseInt(retryAfter) * 1000
            : Math.min(1000 * Math.pow(2, attempt), 30000);
          await wait(waitTime);
          attempt++;
          continue;
        }
        if (!res.ok) {
          clearAuth();
          return;
        }

        const userData = await res.json();
        setUser({ ...userData, auth_type: detected });
        setIsAuthenticated(true);
        return;
      }
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

    // Fetch auth configuration
    getAuthConfig().then((config) => {
      if (mounted) {
        debug("[AuthProvider] Received auth config:", config);
        setAuthConfig(config);
      }
    });

    // Always check once: supports cookie-only sessions.
    checkAuthStatus();

    return () => {
      mounted = false;
    };
  }, [checkAuthStatus]);

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
      const response = await fetch(AUTH_URLS.builtin.login, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(credentials),
        credentials: "include",
      });

      if (!response.ok) {
        const message = await readErrorMessage(response);
        console.error("[AuthProvider] Login failed:", message);
        throw new Error(message || "Login failed");
      }

      await response.json();
      debug("[AuthProvider] Login successful");
      localStorage.setItem("auth_type", "builtin");
      await checkAuthStatus();
    } catch (error) {
      console.error("[AuthProvider] Login error:", error);
      throw error;
    }
  };

  const register = async (credentials: RegisterCredentials) => {
    debug("[AuthProvider] Registration attempt");
    try {
      const response = await fetch(AUTH_URLS.builtin.register, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(credentials),
        credentials: "include",
      });

      if (!response.ok) {
        const message = await readErrorMessage(response);
        console.error("[AuthProvider] Registration failed:", message);
        throw new Error(message || "Registration failed");
      }

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
        authType || (localStorage.getItem("auth_type") as "oidc" | "builtin" | null) || "builtin";
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
