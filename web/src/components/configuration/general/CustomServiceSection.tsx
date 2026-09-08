/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import type {
  CustomActionConfig,
  CustomAuthConfig,
  CustomHealthConfig,
  CustomServiceConfig,
  CustomStatConfig
} from "../../../types/service";
import { CustomServiceAuthFields } from "./CustomServiceAuthFields";
import { CustomServiceLoginFields } from "./CustomServiceLoginFields";
import { CustomServiceHealthFields } from "./CustomServiceHealthFields";
import { CustomServiceStatsEditor } from "./CustomServiceStatsEditor";
import { CustomServiceActionsEditor } from "./CustomServiceActionsEditor";
import { CustomServiceJsonTools } from "./CustomServiceJsonTools";

interface CustomServiceSectionProps {
  config: CustomServiceConfig;
  onChange: (config: CustomServiceConfig) => void;
  url: string;
  apiKey: string;
}

const DEFAULT_AUTH: CustomAuthConfig = { mode: "none" };
const DEFAULT_HEALTH: CustomHealthConfig = { method: "GET", path: "" };

export const CustomServiceSection: React.FC<CustomServiceSectionProps> = ({
  config,
  onChange,
  url,
  apiKey,
}) => {
  const setAuth = (auth: CustomAuthConfig) => onChange({ ...config, auth });
  const setLogin = (login: CustomServiceConfig["login"]) =>
    onChange({ ...config, login });
  const setHealth = (health: CustomHealthConfig) => onChange({ ...config, health });
  const setStats = (stats: CustomStatConfig[]) => onChange({ ...config, stats });
  const setActions = (actions: CustomActionConfig[]) => onChange({ ...config, actions });

  return (
    <div className="space-y-6 rounded-md border border-gray-200 dark:border-gray-700 p-4">
      <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">
        Custom service
      </h3>

      <section className="space-y-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
          Authentication
        </h4>
        <CustomServiceAuthFields
          auth={config.auth || DEFAULT_AUTH}
          onChange={setAuth}
          loginEnabled={Boolean(config.login)}
        />
      </section>

      <section>
        <CustomServiceLoginFields login={config.login} onChange={setLogin} />
      </section>

      <section className="space-y-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
          Health check
        </h4>
        <CustomServiceHealthFields
          health={config.health || DEFAULT_HEALTH}
          onChange={setHealth}
        />
      </section>

      <section className="space-y-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
          Stats
        </h4>
        <CustomServiceStatsEditor stats={config.stats || []} onChange={setStats} />
      </section>

      <section className="space-y-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
          Actions
        </h4>
        <CustomServiceActionsEditor actions={config.actions || []} onChange={setActions} />
      </section>

      <section className="space-y-2">
        <h4 className="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
          Import / export / test
        </h4>
        <CustomServiceJsonTools config={config} onImport={onChange} url={url} apiKey={apiKey} />
      </section>
    </div>
  );
};
