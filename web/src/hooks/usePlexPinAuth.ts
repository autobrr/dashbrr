/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "react-hot-toast";

import { api } from "../utils/api";

type PlexPinCreateResponse = {
  pinId: number;
  code: string;
  clientIdentifier: string;
  authUrl: string;
  expiresIn: number;
};

type PlexPinStatusResponse = {
  authorized: boolean;
  authToken?: string;
  expiresIn: number;
};

const defaultPollIntervalMs = 2000;
const defaultAuthTimeoutMs = 5 * 60 * 1000;
const authWindowCloseGraceMs = 8000;

export const usePlexPinAuth = () => {
  const [isAuthenticating, setIsAuthenticating] = useState(false);
  const pollIntervalRef = useRef<number | null>(null);
  const timeoutRef = useRef<number | null>(null);
  const pollNowRef = useRef<(() => Promise<void>) | null>(null);
  const authWindowRef = useRef<Window | null>(null);
  const authWindowClosedAtRef = useRef<number | null>(null);

  const closeAuthWindow = useCallback(() => {
    if (authWindowRef.current && !authWindowRef.current.closed) {
      authWindowRef.current.close();
    }
    authWindowRef.current = null;
    authWindowClosedAtRef.current = null;
  }, []);

  const stop = useCallback(() => {
    if (pollIntervalRef.current) {
      window.clearInterval(pollIntervalRef.current);
      pollIntervalRef.current = null;
    }
    if (timeoutRef.current) {
      window.clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
    pollNowRef.current = null;
    closeAuthWindow();
    setIsAuthenticating(false);
  }, [closeAuthWindow]);

  useEffect(() => stop, [stop]);

  useEffect(() => {
    const onPlexAuthComplete = (event: MessageEvent) => {
      if (event.origin !== window.location.origin) return;
      if (
        typeof event.data !== "object" ||
        event.data === null ||
        (event.data as { type?: string }).type !== "dashbrr:plex-auth-complete"
      ) {
        return;
      }
      if (pollNowRef.current) {
        void pollNowRef.current();
      }
    };

    window.addEventListener("message", onPlexAuthComplete);
    return () => window.removeEventListener("message", onPlexAuthComplete);
  }, []);

  const openAuthPopup = useCallback((authUrl: string): Window | null => {
    const width = 560;
    const height = 760;
    const left = Math.max(0, window.screenX + (window.outerWidth - width) / 2);
    const top = Math.max(0, window.screenY + (window.outerHeight - height) / 2);

    return window.open(
      authUrl,
      "dashbrr-plex-auth",
      `popup=yes,width=${width},height=${height},left=${Math.round(left)},top=${Math.round(top)}`
    );
  }, []);

  const authenticate = useCallback(
    async (onToken: (token: string) => void) => {
      if (isAuthenticating) return;

      setIsAuthenticating(true);
      try {
        const response = await api.post<PlexPinCreateResponse>(
          "/api/plex/auth/pin",
          {
            forwardUrl: `${window.location.origin}/plex-auth-complete.html`,
          }
        );

        if (!response.authUrl || !response.pinId || !response.code || !response.clientIdentifier) {
          throw new Error("Plex auth initialization returned incomplete data");
        }

        closeAuthWindow();
        const authWindow = openAuthPopup(response.authUrl);
        if (!authWindow) {
          throw new Error("Popup blocked. Allow popups for this site and try again.");
        }
        authWindowRef.current = authWindow;
        authWindowClosedAtRef.current = null;
        authWindow.focus();
        toast.success("Plex login opened");

        const poll = async () => {
          try {
            const status = await api.get<PlexPinStatusResponse>(
              `/api/plex/auth/pin/${response.pinId}?code=${encodeURIComponent(
                response.code
              )}&clientIdentifier=${encodeURIComponent(response.clientIdentifier)}`
            );

            if (!status.authorized || !status.authToken) {
              return;
            }

            onToken(status.authToken);
            stop();
            toast.success("Plex token retrieved");
            return;
          } catch (error) {
            stop();
            toast.error(
              error instanceof Error ? error.message : "Failed to complete Plex authentication"
            );
            return;
          }

          if (!authWindowRef.current?.closed) {
            authWindowClosedAtRef.current = null;
            return;
          }

          if (!authWindowClosedAtRef.current) {
            authWindowClosedAtRef.current = Date.now();
            return;
          }

          const closedAt = authWindowClosedAtRef.current;
          if (typeof closedAt !== "number") {
            return;
          }

          if (Date.now() - closedAt! < authWindowCloseGraceMs) {
            return;
          }

          stop();
          toast.error("Plex authentication window closed before completion.");
        };

        pollNowRef.current = poll;
        pollIntervalRef.current = window.setInterval(poll, defaultPollIntervalMs);
        void poll();

        const timeoutMs = response.expiresIn > 0 ? response.expiresIn * 1000 : defaultAuthTimeoutMs;
        timeoutRef.current = window.setTimeout(() => {
          stop();
          toast.error("Plex authentication timed out. Please try again.");
        }, timeoutMs);
      } catch (error) {
        stop();
        throw error;
      }
    },
    [closeAuthWindow, isAuthenticating, openAuthPopup, stop]
  );

  return {
    isAuthenticating,
    authenticate,
  };
};
