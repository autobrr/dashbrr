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

export const usePlexPinAuth = () => {
  const [isAuthenticating, setIsAuthenticating] = useState(false);
  const pollIntervalRef = useRef<number | null>(null);
  const timeoutRef = useRef<number | null>(null);

  const stop = useCallback(() => {
    if (pollIntervalRef.current) {
      window.clearInterval(pollIntervalRef.current);
      pollIntervalRef.current = null;
    }
    if (timeoutRef.current) {
      window.clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
    setIsAuthenticating(false);
  }, []);

  useEffect(() => stop, [stop]);

  const authenticate = useCallback(
    async (onToken: (token: string) => void) => {
      if (isAuthenticating) return;

      setIsAuthenticating(true);
      try {
        const response = await api.post<PlexPinCreateResponse>(
          "/api/plex/auth/pin",
          {
            forwardUrl: window.location.origin,
          }
        );

        if (!response.authUrl || !response.pinId || !response.code || !response.clientIdentifier) {
          throw new Error("Plex auth initialization returned incomplete data");
        }

        window.open(response.authUrl, "_blank", "noopener,noreferrer");
        toast.success("Plex login opened in a new tab");

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
          } catch (error) {
            stop();
            toast.error(
              error instanceof Error ? error.message : "Failed to complete Plex authentication"
            );
          }
        };

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
    [isAuthenticating, stop]
  );

  return {
    isAuthenticating,
    authenticate,
  };
};
