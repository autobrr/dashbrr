/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export function StatsSkeleton({
  rows = 3,
  showRight = true,
}: {
  rows?: number;
  showRight?: boolean;
}) {
  return (
    <div className="space-y-3">
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          className="flex items-center space-x-3 bg-gray-50 dark:bg-gray-700/50 p-3 rounded-lg animate-pulse"
        >
          <div className="min-w-0 flex-1">
            <div className="h-4 bg-gray-200 dark:bg-gray-600 rounded w-3/4 mb-2" />
            <div className="flex space-x-2">
              <div className="h-3 bg-gray-200 dark:bg-gray-600 rounded w-20" />
              <div className="h-3 bg-gray-200 dark:bg-gray-600 rounded w-24" />
            </div>
          </div>
          {showRight && (
            <div className="flex-shrink-0">
              <div className="h-4 bg-gray-200 dark:bg-gray-600 rounded w-16" />
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
