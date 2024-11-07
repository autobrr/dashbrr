/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { createContext } from "react";
import { ConfigurationContextType } from "./types";

export const ConfigurationContext = createContext<ConfigurationContextType | undefined>(undefined);
