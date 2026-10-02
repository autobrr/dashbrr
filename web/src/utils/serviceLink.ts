/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export interface ServiceLinkTarget {
  url?: string | null;
  accessUrl?: string | null;
}

// Placeholder origin used only to split a path into pathname, query, and fragment.
const PATH_ORIGIN = "http://path.invalid";

/**
 * Builds a link to a service for the user's browser. It uses the access URL
 * when it is set, else the URL. It returns null when that URL is empty or is
 * not an http or https URL. It never throws.
 */
export const serviceLink = (
  { url, accessUrl }: ServiceLinkTarget,
  path = "",
  params?: Record<string, string>
): string | null => {
  const base = accessUrl?.trim() || url?.trim();
  if (!base) return null;

  let link: URL;
  try {
    link = new URL(base);
  } catch {
    return null;
  }
  if (link.protocol !== "http:" && link.protocol !== "https:") return null;

  const basePath = link.pathname.replace(/\/+$/, "");
  const relative = new URL(path.replace(/^\/+/, ""), `${PATH_ORIGIN}/`);
  link.pathname = `${basePath}${relative.pathname}`;
  link.search = relative.search;
  link.hash = relative.hash;
  for (const [key, value] of Object.entries(params ?? {})) {
    link.searchParams.set(key, value);
  }
  return link.toString();
};
