/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

// Use different base URLs for development and production
export const API_BASE_URL = import.meta.env.DEV 
  ? ''  // Empty for development to use relative URLs with Vite proxy
  : ''; // Production URL will be set here

// API prefix for all endpoints
export const API_PREFIX = '/api';

import { api } from '../utils/api';

interface ApiResponse {
  success: boolean;
  message?: string;
}

interface PendingRequestsResponse {
  pendingRequests: number;
}

export interface PlexUser {
  id: string;
  title: string;
  thumb: string;
}

export interface PlexPlayer {
  address: string;
  remotePublicAddress: string;
  device: string;
  machineIdentifier: string;
  model: string;
  platform: string;
  platformVersion: string;
  product: string;
  profile: string;
  state: string;
  title: string;
  vendor: string;
  version: string;
}

export interface PlexSession {
  title: string;
  grandparentTitle: string;
  type: string;
  User?: PlexUser;
  Player?: PlexPlayer;
  state: string;
  TranscodeSession?: {
    videoDecision: string;
    audioDecision: string;
    progress: number;
  };
}

interface PlexSessionsResponse {
  MediaContainer: {
    size: number;
    Metadata: PlexSession[];
  };
}

export interface ReleaseStats {
  total_count: number;
  filtered_count: number;
  filter_rejected_count: number;
  push_approved_count: number;
  push_rejected_count: number;
  push_error_count: number;
}

export interface IRCStatus {
  name: string;
  healthy: boolean;
}

export interface MaintainerrCollection {
  id: number;
  title: string;
  deleteAfterDays: number;
  media: {
    id: number;
    collectionId: number;
    plexId: number;
    tmdbId: number;
    addDate: string;
    image_path: string;
    isManual: boolean;
  }[];
}

const buildUrl = (path: string) => {
  // Ensure path starts with /api
  const apiPath = path.startsWith('/api') ? path : `${API_PREFIX}${path}`;
  return apiPath; // Return just the path for development to use Vite proxy
};

// Get the full backend URL for EventSource (which doesn't work with Vite proxy)
export const getBackendUrl = (path: string) => {
  const apiPath = path.startsWith('/api') ? path : `${API_PREFIX}${path}`;
  return import.meta.env.DEV
    ? `http://localhost:8080${apiPath}`  // Development
    : apiPath;                           // Production
};

export const getPlexSessions = async (baseUrl: string, apiKey: string): Promise<PlexSession[]> => {
  try {
    const params = new URLSearchParams({ url: baseUrl, apiKey });
    const response = await api.get<PlexSessionsResponse>(buildUrl(`/plex/sessions?${params}`));
    return response.MediaContainer.Metadata || [];
  } catch (error) {
    console.error('Error fetching Plex sessions:', error);
    throw error;
  }
};

export const getPendingRequests = async (url: string, apiKey: string): Promise<number> => {
  try {
    const params = new URLSearchParams({ url, apiKey });
    const response = await api.get<PendingRequestsResponse>(buildUrl(`/overseerr/pending?${params}`));
    return response.pendingRequests;
  } catch (error) {
    console.error('Error fetching pending requests:', error);
    throw error;
  }
};

export const getAutobrrStats = async (instanceId: string): Promise<ReleaseStats> => {
  try {
    const params = new URLSearchParams({ instanceId });
    const response = await api.get<ReleaseStats>(buildUrl(`/autobrr/stats?${params}`));
    console.log('Autobrr stats response:', response);
    return response;
  } catch (error) {
    console.error('Error fetching autobrr stats:', error);
    throw error;
  }
};

export const getAutobrrIRC = async (instanceId: string): Promise<IRCStatus[]> => {
  try {
    const params = new URLSearchParams({ instanceId });
    const response = await api.get<IRCStatus[]>(buildUrl(`/autobrr/irc?${params}`));
    console.log('Autobrr IRC response:', response);
    return response;
  } catch (error) {
    console.error('Error fetching autobrr IRC status:', error);
    throw error;
  }
};

export const getMaintainerrCollections = async (instanceId: string): Promise<MaintainerrCollection[]> => {
  try {
    const params = new URLSearchParams({ instanceId });
    const response = await api.get<MaintainerrCollection[]>(buildUrl(`/maintainerr/collections?${params}`));
    console.log('Maintainerr collections response:', response);
    return response;
  } catch (error) {
    console.error('Error fetching maintainerr collections:', error);
    throw error;
  }
};

export const triggerWebhookArrs = async (baseUrl: string, apiKey: string): Promise<ApiResponse> => {
  try {
    const response = await api.post<ApiResponse>(buildUrl('/omegabrr/webhook/arrs'), {
      targetUrl: baseUrl,
      apiKey: apiKey
    });
    return { 
      success: true, 
      message: typeof response === 'string' ? response : JSON.stringify(response) 
    };
  } catch (error) {
    console.error('Error triggering ARRs webhook:', error);
    throw error;
  }
};

export const triggerWebhookLists = async (baseUrl: string, apiKey: string): Promise<ApiResponse> => {
  try {
    const response = await api.post<ApiResponse>(buildUrl('/omegabrr/webhook/lists'), {
      targetUrl: baseUrl,
      apiKey: apiKey
    });
    return { 
      success: true, 
      message: typeof response === 'string' ? response : JSON.stringify(response) 
    };
  } catch (error) {
    console.error('Error triggering Lists webhook:', error);
    throw error;
  }
};

export const triggerWebhookAll = async (baseUrl: string, apiKey: string): Promise<ApiResponse> => {
  try {
    const response = await api.post<ApiResponse>(buildUrl('/omegabrr/webhook/all'), {
      targetUrl: baseUrl,
      apiKey: apiKey
    });
    return { 
      success: true, 
      message: typeof response === 'string' ? response : JSON.stringify(response) 
    };
  } catch (error) {
    console.error('Error triggering All webhook:', error);
    throw error;
  }
};
