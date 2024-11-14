/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";

export function CallbackPage() {
  const [error, setError] = useState<string | null>(null);
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { isAuthenticated } = useAuth();

  useEffect(() => {
    const handleCallback = async () => {
      // First check for tokens in URL (in case of direct callback from Auth0)
      const accessToken = searchParams.get("access_token");
      const idToken = searchParams.get("id_token");

      if (accessToken && idToken) {
        // Store tokens
        localStorage.setItem("access_token", accessToken);
        localStorage.setItem("id_token", idToken);
        // Remove tokens from URL
        window.history.replaceState(
          {},
          document.title,
          window.location.pathname
        );
        // Redirect to home
        navigate("/", { replace: true });
        return;
      }

      // If no tokens in URL, check for error
      const error = searchParams.get("error");
      const errorDescription = searchParams.get("error_description");

      if (error) {
        setError(errorDescription || error);
        return;
      }

      // If no tokens and no error, redirect to home
      navigate("/", { replace: true });
    };

    handleCallback();
  }, [searchParams, navigate, isAuthenticated]);

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="max-w-md w-full space-y-8">
          <div className="rounded-md bg-red-50 p-4">
            <div className="flex">
              <div className="ml-3">
                <h3 className="text-sm font-medium text-red-800">
                  Authentication Error
                </h3>
                <div className="mt-2 text-sm text-red-700">
                  <p>{error}</p>
                </div>
                <div className="mt-4">
                  <button
                    onClick={() => navigate("/login")}
                    className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-red-700 bg-red-100 hover:bg-red-200"
                  >
                    Try Again
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-blue-500"></div>
      <span className="ml-2 text-gray-600">Completing authentication...</span>
    </div>
  );
}
