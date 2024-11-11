/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

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

// Helper function to build URLs with base URL
const buildUrl = (path: string) => {
  // Remove any leading/trailing slashes from path
  const cleanPath = path.replace(/^\/+|\/+$/g, '');

  // Ensure path starts with api/
  const apiPath = cleanPath.startsWith('api/') ? cleanPath : `api/${cleanPath}`;

  // In development, return just the path to use Vite proxy
  if (import.meta.env.DEV) {
    return `/${apiPath}`;
  }

  // In production, use the origin and configured base URL from the server
  const origin = window.location.origin;
  const baseUrl = window.__BASE_URL__;

  // Combine all parts ensuring no double slashes
  if (baseUrl && baseUrl !== '/') {
    return `${origin}${baseUrl}/${apiPath}`;
  }
  return `${origin}/${apiPath}`;
};

// Get the full backend URL for EventSource (which doesn't work with Vite proxy)
export const getBackendUrl = (path: string) => {
  // Remove any leading/trailing slashes from path
  const cleanPath = path.replace(/^\/+|\/+$/g, '');

  if (import.meta.env.DEV) {
    return `http://localhost:8080/${cleanPath}`;
  }

  // In production, use the origin and configured base URL
  const origin = window.location.origin;
  const baseUrl = window.__BASE_URL__;

  // Combine all parts ensuring no double slashes
  if (baseUrl && baseUrl !== '/') {
    return `${origin}${baseUrl}/${cleanPath}`;
  }
  return `${origin}/${cleanPath}`;
};

export const getPlexSessions = async (baseUrl: string, apiKey: string): Promise<PlexSession[]> => {
  try {
    const params = new URLSearchParams({ url: baseUrl, apiKey });
    const response = await api.get<PlexSessionsResponse>(buildUrl(`plex/sessions?${params}`));
    return response.MediaContainer.Metadata || [];
  } catch (error) {
    console.error('Error fetching Plex sessions:', error);
    throw error;
  }
};

export const getPendingRequests = async (url: string, apiKey: string): Promise<number> => {
  try {
    const params = new URLSearchParams({ url, apiKey });
    const response = await api.get<PendingRequestsResponse>(buildUrl(`overseerr/pending?${params}`));
    return response.pendingRequests;
  } catch (error) {
    console.error('Error fetching pending requests:', error);
    throw error;
  }
};

export const getAutobrrStats = async (instanceId: string): Promise<ReleaseStats> => {
  try {
    const params = new URLSearchParams({ instanceId });
    const response = await api.get<ReleaseStats>(buildUrl(`autobrr/stats?${params}`));
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
    const response = await api.get<IRCStatus[]>(buildUrl(`autobrr/irc?${params}`));
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
    const response = await api.get<MaintainerrCollection[]>(buildUrl(`maintainerr/collections?${params}`));
    console.log('Maintainerr collections response:', response);
    return response;
  } catch (error) {
    console.error('Error fetching maintainerr collections:', error);
    throw error;
  }
};

export const triggerWebhookArrs = async (baseUrl: string, apiKey: string): Promise<ApiResponse> => {
  try {
    const response = await api.post<ApiResponse>(buildUrl('omegabrr/webhook/arrs'), {
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
    const response = await api.post<ApiResponse>(buildUrl('omegabrr/webhook/lists'), {
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
    const response = await api.post<ApiResponse>(buildUrl('omegabrr/webhook/all'), {
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
