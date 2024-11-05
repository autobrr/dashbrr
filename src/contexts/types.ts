import { ServiceConfig } from "../types/service";

export interface ConfigurationContextType {
  configurations: { [instanceId: string]: ServiceConfig };
  updateConfiguration: (instanceId: string, config: ServiceConfig) => Promise<ServiceConfig>;
  deleteConfiguration: (instanceId: string) => Promise<void>;
  fetchConfigurations: () => Promise<void>;  // This is now forceRefresh
  isLoading: boolean;
  error: string | null;
}
