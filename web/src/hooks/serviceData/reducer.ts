/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { AutobrrReleases, Service, ServiceConfig, ServiceHealth } from "../../types/service";
import {
  applyServicePatch,
  hydrateServicesFromConfigurations,
} from "./merge";

export type ServiceDataState = {
  services: Map<string, Service>;
  isLoading: boolean;
};

export type ServiceDataAction =
  | { type: "set_loading"; isLoading: boolean }
  | { type: "reset" }
  | {
      type: "hydrate_configurations";
      configurations: Record<string, ServiceConfig>;
      latestPatchByInstance: Map<string, Partial<Service>>;
      latestHealthByInstance: Map<string, ServiceHealth>;
    }
  | { type: "apply_patch"; instanceId: string; patch: Partial<Service> }
  | {
      type: "apply_releases";
      instanceId: string;
      releases: AutobrrReleases;
    };

export const initialServiceDataState: ServiceDataState = {
  services: new Map(),
  isLoading: true,
};

export const serviceDataReducer = (
  state: ServiceDataState,
  action: ServiceDataAction
): ServiceDataState => {
  switch (action.type) {
    case "set_loading":
      if (state.isLoading === action.isLoading) {
        return state;
      }
      return {
        ...state,
        isLoading: action.isLoading,
      };

    case "reset":
      return {
        services: new Map(),
        isLoading: false,
      };

    case "hydrate_configurations":
      return {
        services: hydrateServicesFromConfigurations(
          state.services,
          action.configurations,
          action.latestPatchByInstance,
          action.latestHealthByInstance
        ),
        isLoading: false,
      };

    case "apply_patch":
      return {
        ...state,
        services: applyServicePatch(
          state.services,
          action.instanceId,
          action.patch
        ),
      };

    case "apply_releases":
      return {
        ...state,
        services: applyServicePatch(state.services, action.instanceId, {
          releases: action.releases,
        }),
      };

    default:
      return state;
  }
};
