/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ServiceConfig, ServiceConfigInput, ServiceType } from "../types/service";

export interface CreatedServiceConfig extends ServiceConfig {
  instanceId: string;
}

export interface ConfigurationContextType {
  configurations: { [instanceId: string]: ServiceConfig };
  createConfiguration: (type: ServiceType, config: ServiceConfigInput) => Promise<CreatedServiceConfig>;
  updateConfiguration: (instanceId: string, config: ServiceConfigInput) => Promise<ServiceConfig>;
  deleteConfiguration: (instanceId: string) => Promise<void>;
  fetchConfigurations: () => Promise<void>;  // This is now forceRefresh
  isLoading: boolean;
  error: string | null;
}
