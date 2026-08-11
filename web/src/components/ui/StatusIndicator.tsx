/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useTranslation } from "react-i18next";

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

type StatusDisplay = {
  text: string;
  color: string;
  icon: string;
};

// List of headers that should receive warning styling
const WARNING_HEADERS = [
  "Queue warnings",
  "Indexers unavailable due to failures",
  "Autobrr is running but reports unhealthy IRC connections",
  "Autobrr is running but stats check failed",
];

// List of headers that should receive error styling
const ERROR_HEADERS = ["Service Error", "Error", "Connection issues detected"];

const VIDEO_EXTENSIONS = [
  ".mkv",
  ".mp4",
  ".avi",
  ".mov",
  ".wmv",
  ".flv",
  ".iso",
  ".m4v",
];
const QUALITY_TAGS = ["720p", "1080p", "1440p", "2160p", "4k", "hd", "sd"];
const RELEASE_TYPES = ["webrip", "web-dl", "bluray", "dvd", "hdrip"];

const STATUS_DISPLAY_MAP: Partial<Record<StatusType, StatusDisplay>> = {
  online: { text: "Healthy", color: "text-green-500 dark:text-green-400", icon: "✓" },
  loading: { text: "Checking", color: "text-blue-500 dark:text-blue-400", icon: "⟳" },
  pending: { text: "Pending", color: "text-blue-500 dark:text-blue-400", icon: "○" },
  warning: { text: "Warning", color: "text-amber-500 dark:text-amber-300", icon: "⚠" },
  offline: { text: "Error", color: "text-red-500 dark:text-red-400", icon: "⚠" },
  error: { text: "Error", color: "text-red-500 dark:text-red-400", icon: "⚠" },
};

const UNKNOWN_STATUS_DISPLAY: StatusDisplay = {
  text: "Unknown",
  color: "text-zinc-500 dark:text-zinc-400",
  icon: "?",
};

const getMessageStyle = (status: StatusType, isErrorHeader = false) => {
  const baseStyles = "transition-all duration-200 backdrop-blur-sm";
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
      return `${baseStyles} text-zinc-600 dark:text-zinc-400 bg-zinc-50/90 dark:bg-zinc-900/30 border border-zinc-100 dark:border-zinc-800 shadow-sm`;
  }
};

const isWarningHeader = (title: string): boolean => {
  return WARNING_HEADERS.some((header) => title.startsWith(header));
};

const isErrorHeader = (title: string, status: StatusType): boolean => {
  return (
    ERROR_HEADERS.some((header) => title.startsWith(header)) ||
    (status === "error" && title.startsWith("Error:"))
  );
};

const isReleaseName = (line: string): boolean => {
  const lowered = line.toLowerCase();
  const hasVideoExtension = VIDEO_EXTENSIONS.some((extension) =>
    lowered.includes(extension)
  );
  const hasQualityTag = QUALITY_TAGS.some((quality) => lowered.includes(quality));
  const hasReleaseType = RELEASE_TYPES.some((releaseType) =>
    lowered.includes(releaseType)
  );

  return hasVideoExtension || (hasReleaseType && hasQualityTag);
};

export const StatusIndicator: React.FC<StatusIndicatorProps> = ({
  status,
  message,
  isInitialLoad = false,
  isConnected = true,
}) => {
  const { t } = useTranslation();

  const effectiveStatus = isInitialLoad ? "loading" : !isConnected ? "disconnected" : status;

  const statusDisplay: StatusDisplay = isInitialLoad
    ? { text: t("status.loading", "Loading"), color: "text-blue-500 dark:text-blue-400", icon: "⟳" }
    : !isConnected
      ? {
        text: t("status.disconnected", "Disconnected"),
        color: "text-amber-500 dark:text-amber-300",
        icon: "⚠",
      }
      : {
        ...(STATUS_DISPLAY_MAP[status] || UNKNOWN_STATUS_DISPLAY),
        text: t(`status.${effectiveStatus}`, (STATUS_DISPLAY_MAP[status] || UNKNOWN_STATUS_DISPLAY).text),
      };

  const shouldShowMessage = message && (status !== "online" || !isConnected);
  const displayMessage = isInitialLoad
    ? t("status.loading", "Loading")
    : !isConnected
      ? t("errors.ERR_CONNECTION", "Connection to server lost")
      : (message?.startsWith("ERR_") ? t(`errors.${message}`, message) : message) || "";

  const formatMessage = (msg: string) => {
    const sections: Record<string, React.ReactNode[]> = {};
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
          const isError = isErrorHeader(currentSection, status);
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
          isErrorHeader(line.split(":")[0], status)
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
        if (isWarningHeader(headerPart) || isErrorHeader(headerPart, status)) {
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
          <span className="text-xs mb-1 font-semibold text-zinc-700 dark:text-zinc-300">
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
        <span className="text-xs font-medium text-zinc-700 dark:text-zinc-100">
          {t("service.status", "Status")}:
        </span>
        <div className={`flex items-center gap-1 ${statusDisplay.color}`}>
          <span className="text-xs pointer-events-none">
            {t(`status.${effectiveStatus}`, statusDisplay.text)}
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
