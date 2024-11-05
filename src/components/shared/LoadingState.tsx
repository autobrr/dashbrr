import React from "react";

interface LoadingStateProps {
  message?: string;
  size?: "sm" | "md" | "lg";
  fullScreen?: boolean;
  variant?: "primary" | "secondary" | "minimal";
}

export const LoadingState: React.FC<LoadingStateProps> = ({
  message = "Loading",
  size = "md",
  fullScreen = false,
  variant = "primary",
}) => {
  const sizeClasses = {
    sm: {
      spinner: "w-4 h-4 border-2",
      text: "text-xs",
      container: "gap-2",
    },
    md: {
      spinner: "w-8 h-8 border-3",
      text: "text-sm",
      container: "gap-3",
    },
    lg: {
      spinner: "w-12 h-12 border-4",
      text: "text-base",
      container: "gap-4",
    },
  };

  const variantClasses = {
    primary: {
      spinner: "border-blue-500/20 border-t-blue-500",
      text: "text-gray-600 dark:text-gray-300",
      backdrop: "bg-white/80 dark:bg-gray-900/80",
    },
    secondary: {
      spinner:
        "border-gray-300/30 border-t-gray-300 dark:border-gray-600/30 dark:border-t-gray-600",
      text: "text-gray-500 dark:text-gray-400",
      backdrop: "bg-gray-50/90 dark:bg-gray-800/90",
    },
    minimal: {
      spinner:
        "border-gray-300/20 border-t-gray-300 dark:border-gray-700/20 dark:border-t-gray-700",
      text: "text-gray-400 dark:text-gray-500",
      backdrop: "bg-transparent",
    },
  };

  const containerClasses = fullScreen
    ? "fixed inset-0 flex items-center justify-center backdrop-blur-sm z-50"
    : "flex items-center justify-center p-4";

  return (
    <div
      className={`${containerClasses} ${variantClasses[variant].backdrop} transition-all duration-200`}
    >
      <div
        className={`flex flex-col items-center ${sizeClasses[size].container}`}
      >
        {/* Spinner */}
        <div className="relative">
          <div
            className={`
              ${sizeClasses[size].spinner}
              ${variantClasses[variant].spinner}
              rounded-full border-solid
              animate-spin
              transition-all duration-200
            `}
          />
          {/* Optional pulse effect */}
          <div
            className={`
              absolute inset-0
              rounded-full
              animate-ping
              opacity-20
              ${
                variant === "primary"
                  ? "bg-blue-500"
                  : "bg-gray-400 dark:bg-gray-600"
              }
            `}
          />
        </div>

        {/* Message */}
        {message && (
          <div className="relative">
            <p
              className={`
                ${sizeClasses[size].text}
                ${variantClasses[variant].text}
                font-medium
                transition-all duration-200
                animate-pulse
              `}
            >
              {message}
              <span className="inline-block animate-bounce">...</span>
            </p>
          </div>
        )}
      </div>
    </div>
  );
};

export default LoadingState;
