/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback } from "react";
import { useUIPreferences } from "./useUIPreferences";

export const useCollapsiblePreference = (
  key: string,
  defaultExpanded = true
) => {
  const { getCollapsed, setCollapsed } = useUIPreferences();
  const defaultCollapsed = !defaultExpanded;
  const isCollapsed = getCollapsed(key, defaultCollapsed);
  const isExpanded = !isCollapsed;

  const setIsExpanded = useCallback(
    (expanded: boolean) => setCollapsed(key, !expanded),
    [key, setCollapsed]
  );

  const toggle = useCallback(() => {
    void setCollapsed(key, !isCollapsed);
  }, [isCollapsed, key, setCollapsed]);

  return {
    isExpanded,
    setIsExpanded,
    toggle,
  };
};

