/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { OverseerrMediaRequest } from "../../../types/service";
import { serviceLink, type ServiceLinkTarget } from "../../../utils/serviceLink";

/**
 * Builds the link for a request title. It uses the Radarr or Sonarr link from
 * Overseerr when it is set, else the item page in Overseerr. It returns null
 * when neither link is available.
 */
export const requestLink = (
  service: ServiceLinkTarget,
  media: Pick<OverseerrMediaRequest["media"], "mediaType" | "tmdbId" | "serviceUrl">
): string | null => {
  if (media.serviceUrl) return media.serviceUrl;
  if (!media.tmdbId) return null;
  const path = media.mediaType === "tv" ? "tv" : "movie";
  return serviceLink(service, `${path}/${media.tmdbId}`);
};
