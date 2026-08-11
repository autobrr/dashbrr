/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { SeerrMediaRequest } from "../../../types/service";

export type StatusTone = "pending" | "success" | "error" | "neutral";

export type StatusMeta = {
  label: string;
  color: string;
  tone: StatusTone;
};

// Source of truth: seerr/server/constants/media.ts
export const SEERR_REQUEST_STATUS = {
  PENDING: 1,
  APPROVED: 2,
  DECLINED: 3,
  FAILED: 4,
  COMPLETED: 5,
} as const;

// Source of truth: seerr/server/constants/media.ts
export const SEERR_MEDIA_STATUS = {
  UNKNOWN: 1,
  PENDING: 2,
  PROCESSING: 3,
  PARTIALLY_AVAILABLE: 4,
  AVAILABLE: 5,
  BLACKLISTED: 6,
  DELETED: 7,
} as const;

export const REQUEST_STATUS_META: Record<number, StatusMeta> = {
  [SEERR_REQUEST_STATUS.PENDING]: {
    label: "Pending",
    color: "text-yellow-500",
    tone: "pending",
  },
  [SEERR_REQUEST_STATUS.APPROVED]: {
    label: "Approved",
    color: "text-green-500",
    tone: "success",
  },
  [SEERR_REQUEST_STATUS.DECLINED]: {
    label: "Declined",
    color: "text-red-500",
    tone: "error",
  },
  [SEERR_REQUEST_STATUS.FAILED]: {
    label: "Failed",
    color: "text-red-500",
    tone: "error",
  },
  [SEERR_REQUEST_STATUS.COMPLETED]: {
    label: "Completed",
    color: "text-green-500",
    tone: "success",
  },
};

export const MEDIA_STATUS_META: Record<number, StatusMeta> = {
  [SEERR_MEDIA_STATUS.PENDING]: {
    label: "Pending",
    color: "text-yellow-500",
    tone: "pending",
  },
  [SEERR_MEDIA_STATUS.PROCESSING]: {
    label: "Processing",
    color: "text-blue-400",
    tone: "neutral",
  },
  [SEERR_MEDIA_STATUS.PARTIALLY_AVAILABLE]: {
    label: "Partially Available",
    color: "text-blue-400",
    tone: "neutral",
  },
  [SEERR_MEDIA_STATUS.AVAILABLE]: {
    label: "Available",
    color: "text-green-500",
    tone: "success",
  },
  [SEERR_MEDIA_STATUS.BLACKLISTED]: {
    label: "Blacklisted",
    color: "text-red-500",
    tone: "error",
  },
  [SEERR_MEDIA_STATUS.DELETED]: {
    label: "Deleted",
    color: "text-zinc-400",
    tone: "neutral",
  },
};

const normalizeStatusKey = (raw: string): string =>
  raw.trim().toUpperCase().replace(/[\s-]+/g, "_");

const parseStatusCode = (
  value: unknown,
  aliases: Record<string, number>
): number | undefined => {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }

  if (typeof value !== "string") {
    return undefined;
  }

  const trimmed = value.trim();
  if (trimmed === "") {
    return undefined;
  }

  const parsed = Number.parseInt(trimmed, 10);
  if (!Number.isNaN(parsed)) {
    return parsed;
  }

  const key = normalizeStatusKey(trimmed);
  return aliases[key];
};

const REQUEST_STATUS_ALIASES: Record<string, number> = {
  PENDING: SEERR_REQUEST_STATUS.PENDING,
  APPROVED: SEERR_REQUEST_STATUS.APPROVED,
  DECLINED: SEERR_REQUEST_STATUS.DECLINED,
  FAILED: SEERR_REQUEST_STATUS.FAILED,
  COMPLETED: SEERR_REQUEST_STATUS.COMPLETED,
};

const MEDIA_STATUS_ALIASES: Record<string, number> = {
  UNKNOWN: SEERR_MEDIA_STATUS.UNKNOWN,
  PENDING: SEERR_MEDIA_STATUS.PENDING,
  PROCESSING: SEERR_MEDIA_STATUS.PROCESSING,
  PARTIALLY_AVAILABLE: SEERR_MEDIA_STATUS.PARTIALLY_AVAILABLE,
  PARTIALLYAVAILABLE: SEERR_MEDIA_STATUS.PARTIALLY_AVAILABLE,
  AVAILABLE: SEERR_MEDIA_STATUS.AVAILABLE,
  BLACKLISTED: SEERR_MEDIA_STATUS.BLACKLISTED,
  DELETED: SEERR_MEDIA_STATUS.DELETED,
};

export const getRequestStatus = (value: unknown): number | undefined =>
  parseStatusCode(value, REQUEST_STATUS_ALIASES);

export const getMediaStatus = (value: unknown): number | undefined =>
  parseStatusCode(value, MEDIA_STATUS_ALIASES);

export type ResolvedRequestStatus = {
  meta: StatusMeta;
  isFallback: boolean;
  requestStatus?: number;
  mediaStatus?: number;
};

export const resolveRequestStatus = (
  request: Pick<SeerrMediaRequest, "status" | "media">
): ResolvedRequestStatus => {
  const requestStatus = getRequestStatus(request.status);
  const mediaStatus = getMediaStatus(request.media?.status);

  if (
    requestStatus === SEERR_REQUEST_STATUS.DECLINED ||
    requestStatus === SEERR_REQUEST_STATUS.FAILED ||
    requestStatus === SEERR_REQUEST_STATUS.PENDING
  ) {
    const meta = REQUEST_STATUS_META[requestStatus];
    if (meta) {
      return { meta, isFallback: false, requestStatus, mediaStatus };
    }
  }

  if (
    mediaStatus !== undefined &&
    mediaStatus !== SEERR_MEDIA_STATUS.UNKNOWN
  ) {
    const meta = MEDIA_STATUS_META[mediaStatus];
    if (meta) {
      return { meta, isFallback: true, requestStatus, mediaStatus };
    }
  }

  if (requestStatus !== undefined) {
    const meta = REQUEST_STATUS_META[requestStatus];
    if (meta) {
      return { meta, isFallback: false, requestStatus, mediaStatus };
    }
  }

  if (mediaStatus !== undefined) {
    if (mediaStatus === SEERR_MEDIA_STATUS.UNKNOWN) {
      return {
        meta: {
          label: "Requested",
          color: "text-blue-400",
          tone: "neutral",
        },
        isFallback: true,
        requestStatus,
        mediaStatus,
      };
    }

    const meta = MEDIA_STATUS_META[mediaStatus];
    if (meta) {
      return { meta, isFallback: true, requestStatus, mediaStatus };
    }
  }

  return {
    meta: {
      label: "Requested",
      color: "text-blue-400",
      tone: "neutral",
    },
    isFallback: false,
    requestStatus,
    mediaStatus,
  };
};

