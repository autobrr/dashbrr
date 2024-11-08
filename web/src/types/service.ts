/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export type ServiceStatus = 'online' | 'offline' | 'warning' | 'error' | 'loading' | 'pending' | 'unknown';

export type ServiceType = 'autobrr' | 'omegabrr' | 'radarr' | 'sonarr' | 'prowlarr'| 'overseerr' | 'plex' | 'tailscale' | 'maintainerr' | 'other';

export interface ServiceHealth {
  status: ServiceStatus;
  message: string;
  extras?: Record<string, unknown>;
}

// Base Service interface
export interface Service {
  id: string;
  instanceId: string;
  name: string;
  displayName: string;
  type: ServiceType;
  status: ServiceStatus;
  url: string;
  apiKey?: string;
  lastChecked?: Date;
  responseTime?: number;
  healthEndpoint?: string;
  message?: string;
  updateAvailable?: boolean;
  version?: string;
  retryCount?: number;
  stats?: ServiceStats;
  details?: ServiceDetails;
  health?: ServiceHealth;
}

export interface ServiceConfig {
  url: string;
  apiKey?: string;
  displayName: string;
}

// Autobrr Types
export interface AutobrrStats {
  total_count: number;
  filtered_count: number;
  filter_rejected_count: number;
  push_approved_count: number;
  push_rejected_count: number;
  push_error_count: number;
}

export interface AutobrrIRC {
  name: string;
  healthy: boolean;
}

// Maintainerr Types
export interface MaintainerrMedia {
  id: number;
  collectionId: number;
  plexId: number;
  tmdbId: number;
  addDate: string;
  image_path: string;
  isManual: boolean;
}

export interface MaintainerrCollection {
  id: number;
  title: string;
  deleteAfterDays: number;
  isActive: boolean;
  media: MaintainerrMedia[];
}

// Plex Types
export interface PlexUser {
  id: string;
  title: string;
}

export interface PlexPlayer {
  remotePublicAddress: string;
  product: string;
  device: string;
}

export interface PlexTranscodeSession {
  videoDecision: string;
  audioDecision: string;
  progress: number;
}

export interface PlexSession {
  type: string;
  title: string;
  grandparentTitle?: string;
  User?: PlexUser;
  Player?: PlexPlayer;
  TranscodeSession?: PlexTranscodeSession;
}

// Overseerr Types
export interface OverseerrMediaRequest {
  id: number;
  status: number;
  createdAt: string;
  updatedAt: string;
  media: {
    id: number;
    mediaType: string;
    tmdbId: number;
    tvdbId: number;
    status: number;
    requests: string[];
    createdAt: string;
    updatedAt: string;
    serviceUrl?: string; // fetched from Radarr/Sonarr
    title?: string; // fetched from Radarr/Sonarr
    externalServiceId?: number; // fetched from Radarr/Sonarr
    externalServiceSlug?: string;
  };
  requestedBy: {
    id: number;
    email: string;
    username: string;
    plexToken: string;
    plexUsername: string;
    userType: number;
    permissions: number;
    avatar: string;
    createdAt: string;
    updatedAt: string;
    requestCount: number;
  };
  modifiedBy: {
    id: number;
    email: string;
    username: string;
    plexToken: string;
    plexUsername: string;
    userType: number;
    permissions: number;
    avatar: string;
    createdAt: string;
    updatedAt: string;
    requestCount: number;
  };
  is4k: boolean;
  serverId: number;
  profileId: number;
  rootFolder: string;
}

export interface OverseerrStats {
  pendingCount: number;
  requests: OverseerrMediaRequest[];
  version?: string;
  status?: number;
  updateAvailable?: boolean;
}

// Sonarr Types
export interface SonarrQueueItem {
  id: number;
  title: string;
  status: string;
  indexer?: string;
  customFormatScore: number;
  downloadClient: string;
  timeLeft?: string;
  trackedDownloadState?: string;
  trackedDownloadStatus?: string;
  errorMessage?: string;
}

export interface SonarrQueue {
  totalRecords: number;
  records: SonarrQueueItem[];
}

export interface SonarrStats {
  episodeCount: number;
  episodeFileCount: number;
  monitored: number;
  unmonitored: number;
  queuedCount: number;
  missingCount: number;
}

// Radarr Types
export interface RadarrMovie {
  title: string;
  originalTitle: string;
  year: number;
  folderPath: string;
  customFormats: RadarrCustomFormat[];
}

export interface RadarrCustomFormat {
  id: number;
  name: string;
}

export interface RadarrQueueItem {
  id: number;
  title: string;
  status: string;
  indexer?: string;
  customFormatScore: number;
  downloadClient: string;
  timeLeft?: string;
  trackedDownloadState?: string;
  trackedDownloadStatus?: string;
  errorMessage?: string;
  movie: RadarrMovie;
}

export interface RadarrQueue {
  totalRecords: number;
  records: RadarrQueueItem[];
}

// Prowlarr Types
export interface ProwlarrIndexer {
  id: number;
  name: string;
  enable: boolean;
  priority: number;
}

export interface ProwlarrStats {
  grabCount: number;
  failCount: number;
  indexerCount: number;
}

// Omegabrr Types
export interface OmegabrrWebhookStatus {
  arrs: boolean;
  lists: boolean;
}

// Service Stats Union Type
export interface ServiceStats {
  autobrr?: AutobrrStats;
  maintainerr?: {
    collections: MaintainerrCollection[];
  };
  plex?: {
    sessions: PlexSession[];
  };
  overseerr?: OverseerrStats;
  sonarr?: {
    queue: SonarrQueue;
  };
  radarr?: {
    queue: RadarrQueue;
  };
  prowlarr?: {
    stats: ProwlarrStats;
    indexers: ProwlarrIndexer[];
  };
  omegabrr?: {
    webhookStatus: OmegabrrWebhookStatus;
  };
}

// Service Details Union Type
export interface ServiceDetails {
  autobrr?: {
    irc: AutobrrIRC[];
  };
  omegabrr?: {
    webhookStatus: OmegabrrWebhookStatus;
  };
  plex?: {
    activeStreams: number;
    transcoding: number;
  };
  maintainerr?: {
    activeCollections: number;
    totalMedia: number;
  };
  overseerr?: {
    lastRequestDate?: Date;
    totalRequests?: number;
  };
  sonarr?: {
    queueCount: number;
    monitored: number;
  };
  radarr?: {
    queueCount: number;
  };
  prowlarr?: {
    activeIndexers: number;
    totalGrabs: number;
  };
}
