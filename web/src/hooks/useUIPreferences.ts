/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useContext } from "react";
import { UIPreferencesContext } from "../contexts/UIPreferencesContext";

export const useUIPreferences = () => {
  const context = useContext(UIPreferencesContext);
  if (!context) {
    throw new Error("useUIPreferences must be used within UIPreferencesProvider");
  }
  return context;
};

