import React from "react";

interface StatsLoadingSkeletonProps {
  className?: string;
  size?: "sm" | "md" | "lg";
}

export const StatsLoadingSkeleton: React.FC<StatsLoadingSkeletonProps> = ({
  className = "",
  size = "md",
}) => {
  const sizes = {
    sm: {
      label: "h-4 w-20",
      value: "h-3 w-14",
      gap: "space-y-1",
    },
    md: {
      label: "h-6 w-24",
      value: "h-4 w-16",
      gap: "space-y-1.5",
    },
    lg: {
      label: "h-7 w-32",
      value: "h-5 w-20",
      gap: "space-y-2",
    },
  };

  return (
    <div className={`relative overflow-hidden ${className}`}>
      {/* Shimmer effect overlay */}
      <div className="absolute inset-0">
        <div className="animate-shimmer absolute inset-0 -translate-x-full bg-gradient-to-r from-transparent via-white/10 dark:via-white/5 to-transparent" />
      </div>

      {/* Content */}
      <div className={`relative ${sizes[size].gap}`}>
        <div
          className={`${sizes[size].label} bg-gray-200/80 dark:bg-gray-700/80 rounded animate-pulse`}
        />
        <div
          className={`${sizes[size].value} bg-gray-200/60 dark:bg-gray-700/60 rounded animate-pulse`}
        />
      </div>

      {/* Optional gradient border effect */}
      <div
        className="absolute inset-0 rounded pointer-events-none"
        aria-hidden="true"
      >
        <div className="absolute inset-[-1px] rounded bg-gradient-to-b from-white/5 to-white/[0.01] dark:from-white/3 dark:to-white/[0.005]" />
      </div>
    </div>
  );
};

export default StatsLoadingSkeleton;
