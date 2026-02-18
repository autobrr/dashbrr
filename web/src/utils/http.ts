/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export async function readErrorMessage(response: Response): Promise<string> {
  const contentType = response.headers.get("content-type") || "";

  if (contentType.includes("application/json")) {
    try {
      const data: unknown = await response.json();
      if (typeof data === "string") return data;

      if (data && typeof data === "object") {
        const obj = data as Record<string, unknown>;
        if (typeof obj.error === "string") return obj.error;
        if (typeof obj.message === "string") return obj.message;
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
