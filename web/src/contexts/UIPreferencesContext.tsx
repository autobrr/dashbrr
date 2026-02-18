/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { createContext, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "../hooks/useAuth";
import { api } from "../utils/api";

type CollapsePreferences = Record<string, boolean>;

interface UIPreferencesContextValue {
  getCollapsed: (key: string, defaultCollapsed?: boolean) => boolean;
  setCollapsed: (key: string, collapsed: boolean) => Promise<void>;
}

const UIPreferencesContext = createContext<UIPreferencesContextValue | undefined>(undefined);

export const UIPreferencesProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { isAuthenticated } = useAuth();
  const [collapsePreferences, setCollapsePreferences] = useState<CollapsePreferences>({});
  const collapsePreferencesRef = useRef(collapsePreferences);
  const loadedRef = useRef(false);

  useEffect(() => {
    collapsePreferencesRef.current = collapsePreferences;
  }, [collapsePreferences]);

  useEffect(() => {
    if (!isAuthenticated) {
      loadedRef.current = false;
      setCollapsePreferences({});
      return;
    }

    if (loadedRef.current) {
      return;
    }

    let active = true;

    const fetchCollapsePreferences = async () => {
      try {
        const data = await api.get<CollapsePreferences>("/ui/preferences/collapse");
        if (!active) {
          return;
        }
        setCollapsePreferences(data);
        loadedRef.current = true;
      } catch (error) {
        if (import.meta.env.DEV) {
          console.error("Failed to fetch UI collapse preferences", error);
        }
      }
    };

    void fetchCollapsePreferences();

    return () => {
      active = false;
    };
  }, [isAuthenticated]);

  const setCollapsed = useCallback(async (key: string, collapsed: boolean) => {
    const previous = collapsePreferencesRef.current[key];

    setCollapsePreferences((current) => {
      if (current[key] === collapsed) {
        return current;
      }
      return { ...current, [key]: collapsed };
    });

    try {
      await api.put("/ui/preferences/collapse", { key, collapsed });
    } catch (error) {
      setCollapsePreferences((current) => {
        const next = { ...current };
        if (previous === undefined) {
          delete next[key];
        } else {
          next[key] = previous;
        }
        return next;
      });
      throw error;
    }
  }, []);

  const getCollapsed = useCallback(
    (key: string, defaultCollapsed = false) =>
      collapsePreferences[key] ?? defaultCollapsed,
    [collapsePreferences]
  );

  const value = useMemo<UIPreferencesContextValue>(
    () => ({
      getCollapsed,
      setCollapsed,
    }),
    [getCollapsed, setCollapsed]
  );

  return (
    <UIPreferencesContext.Provider value={value}>
      {children}
    </UIPreferencesContext.Provider>
  );
};

export { UIPreferencesContext };

