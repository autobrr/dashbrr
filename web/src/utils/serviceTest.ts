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
  const result = await api.post<ServiceTestResult>(
    `/health/${encodeURIComponent(instanceId)}`,
    { url: input.url, apiKey: input.apiKey || undefined }
  );
  if (result.status === "error" || result.status === "offline") {
    throw new Error(result.message || `The service reported ${result.status}`);
  }
};
