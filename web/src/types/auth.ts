/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { AuthConfig } from "../config/auth";

export interface LoginCredentials {
  username: string;
  password: string;
}

export interface RegisterCredentials extends LoginCredentials {
  email: string;
}

export interface AuthState {
  isAuthenticated: boolean;
  loading: boolean;
  authConfig: AuthConfig | null;
}

export interface AuthContextType extends AuthState {
  login: (credentials?: LoginCredentials) => Promise<void>;
  register: (credentials: RegisterCredentials) => Promise<void>;
  logout: () => Promise<void>;
  loginWithOIDC: () => void;
}

export interface AuthError {
  message: string;
  code?: string;
}
