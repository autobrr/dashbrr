/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import type { Service } from "../types/service";

export function combineServiceMessage(service: Service): string | undefined {
  const base = service.message;
  const health = service.health?.message;

  if (health) {
    return base ? `${base}\n${health}` : health;
  }

  return base;
}
