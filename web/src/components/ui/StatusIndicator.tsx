/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

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
  "Autobrr is running but reports unhealthy IRC connections",
  "Autobrr is running but stats check failed",
];

// List of headers that should receive error styling
const ERROR_HEADERS = ["Service Error", "Error", "Connection issues detected"];

export const StatusIndicator: React.FC<StatusIndicatorProps> = ({
  status,
  message,
  isInitialLoad = false,
  isConnected = true,
}) => {
  const getMessageStyle = (status: StatusType, isErrorHeader = false) => {
    const baseStyles = "transition-all duration-200 backdrop-blur-sm";

    // Force error styling if it's an error header or status is error/offline
    if (isErrorHeader || status === "error" || status === "offline") {
      return `${baseStyles} text-red-600 dark:text-red-400 bg-red-50/90 dark:bg-red-900/30 border border-red-100 dark:border-red-900/50 shadow-sm shadow-red-100/50 dark:shadow-red-900/30`;
    }

    switch (status) {
      case "online":
        return `${baseStyles} text-green-600 dark:text-green-400 bg-green-50/90 dark:bg-green-900/30 border border-green-100 dark:border-green-900/50 shadow-sm shadow-green-100/50 dark:shadow-green-900/30`;
      case "warning":
        return `${baseStyles} text-amber-500 dark:text-amber-300 bg-amber-50/90 dark:bg-amber-900/20 border border-amber-100 dark:border-amber-800/40 shadow-sm shadow-amber-100/50 dark:shadow-amber-900/20`;
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
        color: "text-amber-500 dark:text-amber-300",
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
          color: "text-amber-500 dark:text-amber-300",
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

  const isErrorHeader = (title: string): boolean => {
    return (
      ERROR_HEADERS.some((header) => title.startsWith(header)) ||
      (status === "error" && title.startsWith("Error:"))
    );
  };

  const isReleaseName = (line: string): boolean => {
    // Define common video file extensions
    const videoExtensions = [
      ".mkv",
      ".mp4",
      ".avi",
      ".mov",
      ".wmv",
      ".flv",
      ".iso",
      ".m4v",
    ];

    const qualityTags = ["720p", "1080p", "1440p", "2160p", "4k", "HD", "SD"];

    const hasVideoExtension = videoExtensions.some((extension) =>
      line.includes(extension)
    );

    const hasQualityTag = qualityTags.some((quality) => line.includes(quality));
    const hasReleaseType =
      line.includes("WEBRip") ||
      line.includes("WEB-DL") ||
      line.includes("BluRay") ||
      line.includes("DVD") ||
      line.includes("HDRip");

    return hasVideoExtension || (hasReleaseType && hasQualityTag);
  };

  const formatMessage = (msg: string) => {
    const sections: { [key: string]: React.ReactNode[] } = {};
    let currentSection = "";
    let currentRelease = "";
    let currentContent: React.ReactNode[] = [];
    let listItems: string[] = [];

    const lines = msg.split("\n").filter((line) => line.trim());

    const addListItems = () => {
      if (listItems.length > 0) {
        currentContent.push(
          <ul
            key={`list-${currentContent.length}`}
            className="list-disc ml-6 space-y-1"
          >
            {listItems.map((item, idx) => (
              <li key={idx} className="text-current opacity-90">
                {item}
              </li>
            ))}
          </ul>
        );
        listItems = [];
      }
    };

    const addToSection = () => {
      addListItems();
      if (currentSection) {
        if (!sections[currentSection]) {
          sections[currentSection] = [];
        }
        if (currentContent.length > 0) {
          const isError = isErrorHeader(currentSection);
          if (currentRelease) {
            // For release-based content
            sections[currentSection].push(
              <div
                key={`${currentSection}-${currentRelease}`}
                className={`text-xs p-2 mb-4 rounded-lg ${getMessageStyle(
                  status,
                  isError
                )}`}
              >
                <div className="text-amber-500 dark:text-amber-300 font-medium mb-2 overflow-hidden">
                  {currentRelease}
                </div>
                <div className="space-y-1">{currentContent}</div>
              </div>
            );
          } else {
            // For non-release content
            sections[currentSection].push(
              <div
                key={`${currentSection}-${sections[currentSection].length}`}
                className={`text-xs p-2 rounded-lg ${getMessageStyle(
                  status,
                  isError
                )}`}
              >
                <div className="space-y-1">{currentContent}</div>
              </div>
            );
          }
        }
      }
      currentContent = [];
    };

    // If no sections are defined in the message, create a default error section
    if (
      status === "error" &&
      !lines.some(
        (line) =>
          isWarningHeader(line.split(":")[0]) ||
          isErrorHeader(line.split(":")[0])
      )
    ) {
      currentSection = "Error";
      currentContent.push(
        <div key="error-message" className="mb-1">
          {msg}
        </div>
      );
      addToSection();
    } else {
      lines.forEach((line, index) => {
        const headerPart = line.split(":")[0].trim();
        if (isWarningHeader(headerPart) || isErrorHeader(headerPart)) {
          addToSection();
          currentSection = headerPart;
          currentRelease = "";
        } else if (isReleaseName(line)) {
          if (currentRelease) {
            addToSection();
          }
          currentRelease = line;
        } else if (line.startsWith("- ")) {
          listItems.push(line.substring(2));
        } else if (line.includes(":")) {
          addListItems();
          const [title, ...rest] = line.split(":");
          currentContent.push(
            <div key={index} className="mb-1">
              <span className="font-medium">{title}:</span>
              {rest.join(":")}
            </div>
          );
        } else if (line.trim()) {
          addListItems();
          currentContent.push(
            <div key={index} className="mb-1">
              {line}
            </div>
          );
        }
      });

      addToSection();
    }

    return Object.entries(sections).map(([sectionTitle, content], idx) => (
      <div key={idx} className="mb-4">
        <div className="flex items-center gap-2 mb-2">
          <span className="text-xs mb-1 font-semibold text-gray-700 dark:text-gray-300">
            {sectionTitle}:
          </span>
        </div>
        {content}
      </div>
    ));
  };

  return (
    <div className="space-y-2 transition-all duration-200 mb-1">
      <div className="flex items-center gap-1.5 select-none pb-2">
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
        <div className="space-y-2">
          {formatMessage(displayMessage)}
          {(status === "loading" || isInitialLoad) && (
            <span className="inline-block animate-pulse ml-1">...</span>
          )}
        </div>
      )}
    </div>
  );
};
