/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { ServiceConfigInput } from "../types/service";
import { api } from "./api";

interface ServiceTestResult {
  status: string;
  message?: string;
}

export const testService = async (
  instanceId: string,
  input: ServiceConfigInput
): Promise<void> => {
  const query = new URLSearchParams({ url: input.url });
  if (input.apiKey) {
    query.set("apiKey", input.apiKey);
  }

  const result = await api.get<ServiceTestResult>(
    `/health/${encodeURIComponent(instanceId)}?${query}`
  );
  if (result.status === "error" || result.status === "offline") {
    throw new Error(result.message || `The service reported ${result.status}`);
  }
};
