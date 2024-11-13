/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export type ServiceStatus = 'online' | 'offline' | 'warning' | 'error' | 'loading' | 'pending' | 'unknown';

export type ServiceType = 'autobrr' | 'omegabrr' | 'radarr' | 'sonarr' | 'prowlarr'| 'overseerr' | 'plex' | 'tailscale' | 'maintainerr' | 'general' | 'other';

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
  thumb?: string;
}

export interface PlexPlayer {
  address: string;
  device?: string;
  machineIdentifier: string;
  model: string;
  platform: string;
  platformVersion: string;
  product: string;
  profile: string;
  remotePublicAddress: string;
  state: string;
  title: string;
  version: string;
  local: boolean;
  relayed: boolean;
  secure: boolean;
  userID: number;
}

export interface PlexMediaStream {
  audioChannelLayout?: string;
  bitDepth?: number;
  bitrate?: number;
  channels?: number;
  codec: string;
  displayTitle: string;
  extendedDisplayTitle: string;
  id: string;
  samplingRate?: number;
  selected: boolean;
  streamType: number;
  location: string;
}

export interface PlexMediaPart {
  container: string;
  duration: number;
  file: string;
  size: number;
  decision: string;
  selected: boolean;
  streams?: PlexMediaStream[];
}

export interface PlexMedia {
  audioChannels: number;
  audioCodec: string;
  bitrate: number;
  container: string;
  duration: number;
  id: string;
  selected: boolean;
  parts?: PlexMediaPart[];
}

export interface PlexTranscodeSession {
  key: string;
  throttled: boolean;
  complete: boolean;
  progress: number;
  speed: number;
  size: number;
  videoDecision: 'transcode' | 'copy' | 'direct play';
  audioDecision: 'transcode' | 'copy' | 'direct play';
  protocol: string;
  container: string;
  videoCodec: string;
  audioCodec: string;
  width: number;
  height: number;
  transcodeHwRequested: boolean;
  transcodeHwFullPipeline: boolean;
  timeStamp: number;
  maxOffsetAvailable: number;
  minOffsetAvailable: number;
}

export interface PlexSession {
  addedAt: number;
  duration: number;
  grandparentArt?: string;
  grandparentGuid?: string;
  grandparentKey?: string;
  grandparentTitle?: string;
  guid: string;
  key: string;
  parentTitle?: string;
  title: string;
  type: string;
  viewOffset: number;
  sessionKey: string;
  User?: PlexUser;
  Player?: PlexPlayer;
  Media?: PlexMedia[];
  Session?: {
    id: string;
    bandwidth: number;
    location: string;
  };
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
    serviceUrl?: string;
    title?: string;
    externalServiceId?: number;
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
export interface SonarrStatusMessage {
  title: string;
  messages: string[];
}

export interface SonarrQueueItem {
  id: number;
  title: string;
  status: string;
  protocol: string; // "usenet" or "torrent"
  indexer?: string;
  customFormatScore: number;
  downloadClient: string;
  timeLeft?: string;
  trackedDownloadState?: string;
  trackedDownloadStatus?: string;
  errorMessage?: string;
  statusMessages?: SonarrStatusMessage[];
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

export interface RadarrStatusMessage {
  title: string;
  messages: string[];
}

export interface RadarrQueueItem {
  id: number;
  title: string;
  status: string;
  protocol: string; // "usenet" or "torrent"
  indexer?: string;
  customFormatScore: number;
  downloadClient: string;
  timeLeft?: string;
  trackedDownloadState?: string;
  trackedDownloadStatus?: string;
  errorMessage?: string;
  movie: RadarrMovie;
  movieId: number;
  statusMessages?: RadarrStatusMessage[];
}

export interface RadarrQueue {
  totalRecords: number;
  records: RadarrQueueItem[];
}

// Prowlarr Types
export interface ProwlarrIndexer {
  id: number;
  name: string;
  label: string;
  enable: boolean;
  priority: number;
  averageResponseTime: number;
  numberOfGrabs: number;
  numberOfQueries: number;
}

export interface ProwlarrStats {
  grabCount: number;
  failCount: number;
  indexerCount: number;
  numberOfGrabs: number;
  numberOfQueries: number;
}

export interface ProwlarrIndexerStats {
  id: number;
  indexerId: number;
  indexerName: string;
  averageResponseTime: number;
  numberOfQueries: number;
  numberOfGrabs: number;
  numberOfRssQueries: number;
  numberOfAuthQueries: number;
  numberOfFailedQueries: number;
  numberOfFailedGrabs: number;
  numberOfFailedRssQueries: number;
  numberOfFailedAuthQueries: number;
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
    prowlarrIndexerStats: {
      id: number;
      indexers: ProwlarrIndexerStats[];
    };
  }
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
