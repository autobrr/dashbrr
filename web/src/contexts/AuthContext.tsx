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

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Utility function for exponential backoff
const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

async function readApiError(response: Response): Promise<string> {
  const contentType = response.headers.get("content-type") || "";

  if (contentType.includes("application/json")) {
    try {
      const data: unknown = await response.json();
      if (typeof data === "string") return data;

      if (data && typeof data === "object") {
        const obj = data as Record<string, unknown>;
        // Backend commonly returns `{ error: "..." }`
        if (typeof obj.error === "string") {
          return obj.error;
        }
        if (typeof obj.message === "string") {
          return obj.message;
        }
      }

      return JSON.stringify(data);
    } catch {
      // fall through
    }
  }

  try {
    const text = await response.text();
    if (text) return text;
  } catch {
    // ignore
  }

  return `${response.status} ${response.statusText}`.trim();
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null);

  const clearAuth = useCallback(() => {
    console.log("[AuthProvider] Clearing authentication state");
    localStorage.removeItem("access_token");
    localStorage.removeItem("id_token");
    localStorage.removeItem("auth_type");
    setUser(null);
    setIsAuthenticated(false);
  }, []);

  const checkAuthStatus = useCallback(async () => {
    const MAX_RETRIES = 5;
    let attempt = 0;

    console.log("[AuthProvider] Checking auth status");
    setLoading(true);

    try {
      const accessToken = localStorage.getItem("access_token");
      const currentAuthType = localStorage.getItem("auth_type") as
        | "oidc"
        | "builtin"
        | null;

      if (!accessToken || !currentAuthType) {
        console.log("[AuthProvider] No access token or auth type found");
        clearAuth();
        return;
      }

      console.log(
        "[AuthProvider] Verifying token for auth type:",
        currentAuthType
      );

      const verifyUrl =
        currentAuthType === "oidc"
          ? AUTH_URLS.oidc.verify
          : AUTH_URLS.builtin.verify;

      console.log("[AuthProvider] Verifying token at:", verifyUrl);

      while (attempt < MAX_RETRIES) {
        const verifyResponse = await fetch(verifyUrl, {
          headers: {
            Authorization: `Bearer ${accessToken}`,
          },
          credentials: "include",
        });

        if (verifyResponse.status === 429) {
          const retryAfter = verifyResponse.headers.get("Retry-After");
          const waitTime = retryAfter
            ? parseInt(retryAfter) * 1000
            : Math.min(1000 * Math.pow(2, attempt), 30000);
          console.log(
            `[AuthProvider] Rate limited, waiting ${waitTime}ms before retry`
          );
          await wait(waitTime);
          attempt++;
          continue;
        }

        if (!verifyResponse.ok) {
          console.error(
            "[AuthProvider] Token verification failed:",
            verifyResponse.status
          );
          clearAuth();
          return;
        }

        console.log("[AuthProvider] Token verified successfully");
        break;
      }

      if (attempt >= MAX_RETRIES) {
        throw new Error("Auth verification rate-limited (max retries reached)");
      }

      const userInfoUrl =
        currentAuthType === "oidc" ? AUTH_URLS.oidc.userInfo : AUTH_URLS.userInfo;

      console.log("[AuthProvider] Fetching user info from:", userInfoUrl);

      attempt = 0;
      while (attempt < MAX_RETRIES) {
        const userInfoResponse = await fetch(userInfoUrl, {
          headers: {
            Authorization: `Bearer ${accessToken}`,
          },
          credentials: "include",
        });

        if (userInfoResponse.status === 429) {
          const retryAfter = userInfoResponse.headers.get("Retry-After");
          const waitTime = retryAfter
            ? parseInt(retryAfter) * 1000
            : Math.min(1000 * Math.pow(2, attempt), 30000);
          console.log(
            `[AuthProvider] Rate limited, waiting ${waitTime}ms before retry`
          );
          await wait(waitTime);
          attempt++;
          continue;
        }

        if (!userInfoResponse.ok) {
          console.error(
            "[AuthProvider] Failed to get user info:",
            userInfoResponse.status
          );
          clearAuth();
          return;
        }

        const userData = await userInfoResponse.json();
        console.log("[AuthProvider] User info received:", {
          ...userData,
          auth_type: currentAuthType,
        });

        setUser({ ...userData, auth_type: currentAuthType });
        setIsAuthenticated(true);
        console.log("[AuthProvider] Authentication successful");
        return;
      }

      throw new Error("User info rate-limited (max retries reached)");
    } catch (error) {
      console.error("[AuthProvider] Auth check failed:", error);
      clearAuth();
    } finally {
      setLoading(false);
    }
  }, [clearAuth]);

  useEffect(() => {
    let mounted = true;
    console.log("[AuthProvider] Initializing auth provider");

    // Fetch auth configuration
    getAuthConfig().then((config) => {
      if (mounted) {
        console.log("[AuthProvider] Received auth config:", config);
        setAuthConfig(config);
      }
    });

    // Check for OIDC auth tokens in URL (after callback)
    const params = new URLSearchParams(window.location.search);
    const accessToken = params.get("access_token");
    const idToken = params.get("id_token");

    if (accessToken && idToken) {
      console.log("[AuthProvider] Found OIDC tokens in URL");
      // Store tokens and remove them from URL
      localStorage.setItem("access_token", accessToken);
      localStorage.setItem("id_token", idToken);
      localStorage.setItem("auth_type", "oidc");
      window.history.replaceState({}, document.title, window.location.pathname);
    }

    // Only do one initial auth check
    const storedAccessToken = localStorage.getItem("access_token");
    const storedAuthType = localStorage.getItem("auth_type");

    if (storedAccessToken && storedAuthType) {
      checkAuthStatus();
    } else {
      setLoading(false);
    }

    return () => {
      mounted = false;
    };
  }, [checkAuthStatus]);

  const loginWithOIDC = () => {
    console.log("[AuthProvider] Initiating OIDC login");
    if (!authConfig?.methods.oidc) {
      throw new Error("OIDC authentication is not configured");
    }
    clearAuth();
    window.location.href = AUTH_URLS.oidc.login;
  };

  const login = async (credentials?: LoginCredentials) => {
    console.log(
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
        const message = await readApiError(response);
        console.error("[AuthProvider] Login failed:", message);
        throw new Error(message || "Login failed");
      }

      const data = await response.json();
      console.log("[AuthProvider] Login successful");
      localStorage.setItem("access_token", data.access_token);
      localStorage.setItem("auth_type", "builtin");
      await checkAuthStatus();
    } catch (error) {
      console.error("[AuthProvider] Login error:", error);
      throw error;
    }
  };

  const register = async (credentials: RegisterCredentials) => {
    console.log("[AuthProvider] Registration attempt");
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
        const message = await readApiError(response);
        console.error("[AuthProvider] Registration failed:", message);
        throw new Error(message || "Registration failed");
      }

      console.log(
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
    console.log("[AuthProvider] Initiating logout");
    try {
      const currentAuthType = localStorage.getItem("auth_type") as
        | "oidc"
        | "builtin";
      const logoutUrl =
        currentAuthType === "oidc"
          ? AUTH_URLS.oidc.logout
          : AUTH_URLS.builtin.logout;
      const accessToken = localStorage.getItem("access_token");

      console.log(
        "[AuthProvider] Logging out with auth type:",
        currentAuthType
      );

      const response = await fetch(logoutUrl, {
        method: "POST",
        headers: accessToken
          ? {
              Authorization: `Bearer ${accessToken}`,
            }
          : undefined,
        credentials: "include",
      });

      if (!response.ok) {
        console.error("[AuthProvider] Logout request failed:", response.status);
        throw new Error("Logout failed");
      }

      console.log("[AuthProvider] Logout successful");
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
