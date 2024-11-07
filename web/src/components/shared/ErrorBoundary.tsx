/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { Component, ErrorInfo, ReactNode } from "react";
import { XCircleIcon, ArrowPathIcon } from "@heroicons/react/24/outline";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
  onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

interface State {
  hasError: boolean;
  error?: Error;
}

export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("ErrorBoundary caught an error:", error, errorInfo);
    this.props.onError?.(error, errorInfo);
  }

  private handleRetry = () => {
    this.setState({ hasError: false, error: undefined });
  };

  public render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="relative overflow-hidden animate-fadeIn">
          <div className="p-6 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-100 dark:border-red-800/50 shadow-sm">
            {/* Header */}
            <div className="flex items-start gap-3">
              <div className="flex-shrink-0">
                <XCircleIcon className="h-6 w-6 text-red-500 dark:text-red-400" />
              </div>
              <div className="flex-1 min-w-0">
                <h2 className="text-lg font-semibold text-red-800 dark:text-red-200">
                  Something went wrong
                </h2>
                <div className="mt-2 space-y-2">
                  <p className="text-sm text-red-700 dark:text-red-300">
                    {this.state.error?.message ||
                      "An unexpected error occurred"}
                  </p>
                  {this.state.error?.stack && (
                    <pre className="mt-2 p-3 text-xs rounded bg-red-100/50 dark:bg-red-900/30 text-red-800 dark:text-red-200 overflow-auto max-h-32">
                      {this.state.error.stack}
                    </pre>
                  )}
                </div>
              </div>
            </div>

            {/* Actions */}
            <div className="mt-6 flex items-center justify-end gap-3">
              <button
                onClick={() => window.location.reload()}
                className="inline-flex items-center px-4 py-2 text-sm font-medium text-red-700 dark:text-red-200 hover:text-red-800 dark:hover:text-red-100 transition-colors duration-200"
              >
                Reload page
              </button>
              <button
                onClick={this.handleRetry}
                className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-red-600 text-white hover:bg-red-700 dark:bg-red-500 dark:hover:bg-red-600 transition-all duration-200 text-sm font-medium shadow-sm hover:shadow focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 dark:focus:ring-offset-gray-900"
              >
                <ArrowPathIcon className="h-4 w-4" />
                Try again
              </button>
            </div>
          </div>

          {/* Gradient border effect */}
          <div
            className="absolute inset-0 rounded-lg pointer-events-none"
            aria-hidden="true"
          >
            <div className="absolute inset-[-1px] rounded-lg bg-gradient-to-b from-red-500/10 to-red-500/[0.02] dark:from-red-400/5 dark:to-red-400/[0.01]" />
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
