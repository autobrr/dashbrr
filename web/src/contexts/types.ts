/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ServiceConfig, ServiceConfigInput } from "../types/service";

export interface ConfigurationContextType {
  configurations: { [instanceId: string]: ServiceConfig };
  updateConfiguration: (instanceId: string, config: ServiceConfigInput) => Promise<ServiceConfig>;
  deleteConfiguration: (instanceId: string) => Promise<void>;
  fetchConfigurations: () => Promise<void>;  // This is now forceRefresh
  isLoading: boolean;
  error: string | null;
}
