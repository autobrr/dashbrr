/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
} from "react";
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

// Debounce function with proper type handling
function debounce<TArgs extends unknown[], TReturn>(
  func: (...args: TArgs) => Promise<TReturn>,
  waitTime: number
): (...args: TArgs) => Promise<TReturn> {
  let timeout: NodeJS.Timeout;
  let lastCall = 0;

  return (...args: TArgs): Promise<TReturn> => {
    return new Promise((resolve, reject) => {
      const now = Date.now();
      const timeSinceLastCall = now - lastCall;

      clearTimeout(timeout);

      if (timeSinceLastCall >= waitTime) {
        lastCall = now;
        func(...args)
          .then(resolve)
          .catch(reject);
      } else {
        timeout = setTimeout(() => {
          lastCall = Date.now();
          func(...args)
            .then(resolve)
            .catch(reject);
        }, waitTime - timeSinceLastCall);
      }
    });
  };
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [authConfig, setAuthConfig] = useState<AuthConfig | null>(null);
  const [retryCount, setRetryCount] = useState(0);

  const checkAuthStatus = useCallback(async () => {
    console.log("[AuthProvider] Checking auth status");
    try {
      const accessToken = localStorage.getItem("access_token");
      const currentAuthType = localStorage.getItem("auth_type") as
        | "oidc"
        | "builtin"
        | null;

      if (!accessToken || !currentAuthType) {
        console.log("[AuthProvider] No access token or auth type found");
        throw new Error("No access token or auth type");
      }

      console.log(
        "[AuthProvider] Verifying token for auth type:",
        currentAuthType
      );

      // First verify the token
      const verifyUrl =
        currentAuthType === "oidc"
          ? AUTH_URLS.oidc.verify
          : AUTH_URLS.builtin.verify;

      console.log("[AuthProvider] Verifying token at:", verifyUrl);

      const verifyResponse = await fetch(verifyUrl, {
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
        credentials: "include",
      });

      if (!verifyResponse.ok) {
        if (verifyResponse.status === 429) {
          // Rate limit hit - implement exponential backoff
          const retryAfter = verifyResponse.headers.get("Retry-After");
          const waitTime = retryAfter
            ? parseInt(retryAfter) * 1000
            : Math.min(1000 * Math.pow(2, retryCount), 30000);
          console.log(
            `[AuthProvider] Rate limited, waiting ${waitTime}ms before retry`
          );
          await wait(waitTime);
          setRetryCount((prev) => prev + 1);
          return checkAuthStatus(); // Retry after waiting
        }
        console.error(
          "[AuthProvider] Token verification failed:",
          verifyResponse.status
        );
        throw new Error("Token verification failed");
      }

      // Reset retry count on successful verification
      setRetryCount(0);
      console.log("[AuthProvider] Token verified successfully");

      // Then get user info using the appropriate endpoint
      const userInfoUrl =
        currentAuthType === "oidc"
          ? AUTH_URLS.oidc.userInfo
          : AUTH_URLS.userInfo;

      console.log("[AuthProvider] Fetching user info from:", userInfoUrl);

      const userInfoResponse = await fetch(userInfoUrl, {
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
        credentials: "include",
      });

      if (!userInfoResponse.ok) {
        if (userInfoResponse.status === 429) {
          // Rate limit hit - implement exponential backoff
          const retryAfter = userInfoResponse.headers.get("Retry-After");
          const waitTime = retryAfter
            ? parseInt(retryAfter) * 1000
            : Math.min(1000 * Math.pow(2, retryCount), 30000);
          console.log(
            `[AuthProvider] Rate limited, waiting ${waitTime}ms before retry`
          );
          await wait(waitTime);
          setRetryCount((prev) => prev + 1);
          return checkAuthStatus(); // Retry after waiting
        }
        console.error(
          "[AuthProvider] Failed to get user info:",
          userInfoResponse.status
        );
        throw new Error("Failed to get user info");
      }

      // Reset retry count on successful user info fetch
      setRetryCount(0);
      const userData = await userInfoResponse.json();
      console.log("[AuthProvider] User info received:", {
        ...userData,
        auth_type: currentAuthType,
      });

      setUser({ ...userData, auth_type: currentAuthType });
      setIsAuthenticated(true);
      console.log("[AuthProvider] Authentication successful");
    } catch (error) {
      console.error("[AuthProvider] Auth check failed:", error);
      clearAuth();
    } finally {
      setLoading(false);
    }
  }, [retryCount]);

  // Debounce the auth check to prevent too frequent calls
  const debouncedCheckAuth = useCallback(
    debounce(checkAuthStatus, 5000), // 5 second debounce
    [checkAuthStatus]
  );

  useEffect(() => {
    console.log("[AuthProvider] Initializing auth provider");

    // Fetch auth configuration
    getAuthConfig().then((config) => {
      console.log("[AuthProvider] Received auth config:", config);
      setAuthConfig(config);
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
      debouncedCheckAuth();
    } else {
      // Check if we have stored tokens
      const storedAccessToken = localStorage.getItem("access_token");
      const storedAuthType = localStorage.getItem("auth_type") as
        | "oidc"
        | "builtin"
        | null;

      console.log("[AuthProvider] Checking stored auth:", {
        hasStoredToken: !!storedAccessToken,
        storedAuthType,
      });

      if (storedAccessToken && storedAuthType) {
        debouncedCheckAuth();
      } else {
        setLoading(false);
      }
    }
  }, [debouncedCheckAuth]);

  const clearAuth = () => {
    console.log("[AuthProvider] Clearing authentication state");
    localStorage.removeItem("access_token");
    localStorage.removeItem("id_token");
    localStorage.removeItem("auth_type");
    setIsAuthenticated(false);
    setUser(null);
    setRetryCount(0);
  };

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
        const error = await response.json();
        console.error("[AuthProvider] Login failed:", error);
        throw new Error(error.message || "Login failed");
      }

      const data = await response.json();
      console.log("[AuthProvider] Login successful");
      localStorage.setItem("access_token", data.access_token);
      localStorage.setItem("auth_type", "builtin");
      await debouncedCheckAuth();
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
        const error = await response.json();
        console.error("[AuthProvider] Registration failed:", error);
        throw new Error(error.message || "Registration failed");
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
      window.location.href = "/login";
    } catch (error) {
      console.error("[AuthProvider] Logout error:", error);
      clearAuth();
      window.location.href = "/login";
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

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
