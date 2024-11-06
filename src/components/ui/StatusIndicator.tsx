import React from "react";

type StatusType =
  | "online"
  | "offline"
  | "warning"
  | "error"
  | "loading"
  | "unknown"
  | "healthy"
  | "pending";

interface StatusIndicatorProps {
  status: StatusType;
  message?: string;
  isInitialLoad?: boolean;
  isConnected?: boolean;
}

// List of headers that should receive warning styling
const WARNING_HEADERS = [
  "Queue warnings",
  "Indexers unavailable due to failures",
];

export const StatusIndicator: React.FC<StatusIndicatorProps> = ({
  status,
  message,
  isInitialLoad = false,
  isConnected = true,
}) => {
  const getMessageStyle = (status: StatusType) => {
    const baseStyles = "transition-all duration-200 backdrop-blur-sm";

    switch (status) {
      case "online":
        return `${baseStyles} text-green-600 dark:text-green-400 bg-green-50/90 dark:bg-green-900/30 border border-green-100 dark:border-green-900/50 shadow-sm shadow-green-100/50 dark:shadow-green-900/30`;
      case "warning":
        return `${baseStyles} text-yellow-600 dark:text-yellow-400 bg-yellow-50/90 dark:bg-yellow-900/30 border border-yellow-100 dark:border-yellow-900/50 shadow-sm shadow-yellow-100/50 dark:shadow-yellow-900/30`;
      case "offline":
      case "error":
        return `${baseStyles} text-red-600 dark:text-red-400 bg-red-50/90 dark:bg-red-900/30 border border-red-100 dark:border-red-900/50 shadow-sm shadow-red-100/50 dark:shadow-red-900/30`;
      case "loading":
      case "pending":
        return `${baseStyles} text-blue-600 dark:text-blue-400 bg-blue-50/90 dark:bg-blue-900/30 border border-blue-100 dark:border-blue-900/50 shadow-sm shadow-blue-100/50 dark:shadow-blue-900/30`;
      default:
        return `${baseStyles} text-gray-600 dark:text-gray-400 bg-gray-50/90 dark:bg-gray-900/30 border border-gray-100 dark:border-gray-800 shadow-sm`;
    }
  };

  const getStatusDisplay = () => {
    if (isInitialLoad) {
      return {
        text: "Loading",
        color: "text-blue-500 dark:text-blue-400",
        icon: "⟳",
      };
    }

    if (!isConnected) {
      return {
        text: "Disconnected",
        color: "text-yellow-500 dark:text-yellow-400",
        icon: "⚠",
      };
    }

    switch (status) {
      case "online":
        return {
          text: "Healthy",
          color: "text-green-500 dark:text-green-400",
          icon: "✓",
        };
      case "loading":
        return {
          text: "Checking",
          color: "text-blue-500 dark:text-blue-400",
          icon: "⟳",
        };
      case "pending":
        return {
          text: "Pending",
          color: "text-blue-500 dark:text-blue-400",
          icon: "○",
        };
      case "warning":
        return {
          text: "Warning",
          color: "text-yellow-500 dark:text-yellow-400",
          icon: "⚠",
        };
      case "offline":
      case "error":
        return {
          text: "Error",
          color: "text-red-500 dark:text-red-400",
          icon: "⚠",
        };
      default:
        return {
          text: "Unknown",
          color: "text-gray-500 dark:text-gray-400",
          icon: "?",
        };
    }
  };

  const statusDisplay = getStatusDisplay();
  const shouldShowMessage = message && (status !== "online" || !isConnected);
  const displayMessage = isInitialLoad
    ? "Initializing service..."
    : !isConnected
    ? "Connection to server lost"
    : message || "";

  const isWarningHeader = (title: string): boolean => {
    return WARNING_HEADERS.some((header) => title.startsWith(header));
  };

  const formatMessage = (msg: string) => {
    // Split the message into lines
    const lines = msg.split("\n");

    // Initialize an array to hold the JSX elements
    const elements: React.ReactNode[] = [];

    // Temporary array to collect list items
    let listItems: string[] = [];

    lines.forEach((line, index) => {
      if (line.startsWith("- ")) {
        // Collect list items
        listItems.push(line.substring(2));
      } else {
        // If there are accumulated list items, render them first
        if (listItems.length > 0) {
          elements.push(
            <ul key={`ul-${index}`} className="list-disc ml-6 space-y-1 pb-2">
              {listItems.map((item, idx) => (
                <li key={idx} className="text-current opacity-90">
                  {item}
                </li>
              ))}
            </ul>
          );
          listItems = []; // Reset list items
        }

        // Handle lines with ":" if needed
        if (line.includes(":")) {
          const [title, ...rest] = line.split(":");
          const isWarning = isWarningHeader(title.trim());

          if (rest.length === 0) {
            elements.push(
              <div
                key={index}
                className={`${
                  isWarning
                    ? "text-yellow-500 dark:text-yellow-400 tracking-wider mt-4 mb-2"
                    : "font-medium mt-3 mb-2"
                }`}
              >
                {title}:
              </div>
            );
          } else {
            // If there's content after ":", display it normally
            elements.push(
              <div key={index} className="space-y-2">
                <div
                  className={`${
                    isWarning
                      ? "text-yellow-500 dark:text-yellow-400 text-xs font-bold pb-1 tracking-wider"
                      : "font-medium"
                  } mb-2`}
                >
                  {title}:
                </div>
                <div className="ml-1">{rest.join(":")}</div>
              </div>
            );
          }
        } else {
          // Regular lines
          elements.push(<div key={index}>{line}</div>);
        }
      }
    });

    // If any list items remain after the loop, render them
    if (listItems.length > 0) {
      elements.push(
        <ul key="ul-end" className="list-disc ml-6 space-y-1">
          {listItems.map((item, idx) => (
            <li key={idx} className="text-current opacity-90">
              {item}
            </li>
          ))}
        </ul>
      );
    }

    return elements;
  };

  return (
    <div className="space-y-2 transition-all duration-200 mb-4">
      <div className="flex items-center gap-1.5 select-none">
        <span className="text-xs font-medium text-gray-700 dark:text-gray-100">
          Status
        </span>
        <div className={`flex items-center gap-1 ${statusDisplay.color}`}>
          <span className="text-xs pointer-events-none">
            {statusDisplay.text}
          </span>
          <span
            className={`text-xs ${
              status === "loading" || isInitialLoad ? "animate-spin" : ""
            }`}
          >
            {statusDisplay.icon}
          </span>
        </div>
      </div>

      {shouldShowMessage && (
        <div className={`text-xs p-2 rounded-lg ${getMessageStyle(status)}`}>
          <div className="space-y-1 mr-4 font-medium overflow-hidden">
            {formatMessage(displayMessage)}
          </div>
          {(status === "loading" || isInitialLoad) && (
            <span className="inline-block animate-pulse ml-1">...</span>
          )}
        </div>
      )}
    </div>
  );
};
