/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { Service } from "../types/service";

const splitLines = (message: string): string[] =>
  message
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);

const dedupeLines = (lines: string[]): string[] => {
  const seen = new Set<string>();
  const result: string[] = [];

  for (const line of lines) {
    if (seen.has(line)) {
      continue;
    }
    seen.add(line);
    result.push(line);
  }

  return result;
};

export function combineServiceMessage(service: Service): string | undefined {
  const combined = dedupeLines([
    ...splitLines(service.message ?? ""),
    ...splitLines(service.health?.message ?? ""),
  ]);

  if (combined.length > 0) {
    return combined.join("\n");
  }

  return undefined;
}
