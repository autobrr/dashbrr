import React from "react";

export const LoadingSkeleton: React.FC = () => (
  <div className="relative bg-white dark:bg-gray-800 rounded-lg shadow-lg h-full min-h-[220px] overflow-hidden transition-all duration-200 border border-gray-100 dark:border-gray-700">
    {/* Shimmer effect overlay */}
    <div className="absolute inset-0">
      <div className="animate-shimmer absolute inset-0 -translate-x-full bg-gradient-to-r from-transparent via-white/10 dark:via-white/5 to-transparent" />
    </div>

    <div className="relative p-4 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="h-6 w-32 bg-gray-200 dark:bg-gray-700 rounded-md animate-pulse" />
          <div className="h-5 w-16 bg-gray-200/80 dark:bg-gray-700/80 rounded-full animate-pulse" />
        </div>
        <div className="flex items-center gap-2">
          <div className="h-6 w-6 bg-gray-200 dark:bg-gray-700 rounded-full animate-pulse" />
          <div className="h-6 w-6 bg-gray-200 dark:bg-gray-700 rounded-full animate-pulse" />
        </div>
      </div>

      {/* Status section */}
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <div className="h-4 w-14 bg-gray-200/80 dark:bg-gray-700/80 rounded animate-pulse" />
          <div className="h-4 w-20 bg-gray-200/80 dark:bg-gray-700/80 rounded animate-pulse" />
        </div>
        <div className="h-20 bg-gray-200/60 dark:bg-gray-700/60 rounded-lg animate-pulse" />
      </div>

      {/* Content blocks */}
      <div className="space-y-3">
        <div className="flex items-center gap-4">
          <div className="h-8 w-8 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
          <div className="flex-1">
            <div className="h-4 w-3/4 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
          </div>
        </div>
        <div className="flex items-center gap-4">
          <div className="h-8 w-8 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
          <div className="flex-1">
            <div className="h-4 w-2/3 bg-gray-200 dark:bg-gray-700 rounded animate-pulse" />
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="absolute bottom-4 left-4 right-4">
        <div className="space-y-2">
          <div className="h-3 w-24 bg-gray-200/80 dark:bg-gray-700/80 rounded animate-pulse" />
          <div className="h-3 w-32 bg-gray-200/80 dark:bg-gray-700/80 rounded animate-pulse" />
        </div>
      </div>
    </div>

    {/* Gradient border effect */}
    <div
      className="absolute inset-0 rounded-lg pointer-events-none"
      aria-hidden="true"
    >
      <div className="absolute inset-[-1px] rounded-lg bg-gradient-to-b from-white/10 to-white/[0.02] dark:from-white/5 dark:to-white/[0.01]" />
    </div>
  </div>
);

export default LoadingSkeleton;
