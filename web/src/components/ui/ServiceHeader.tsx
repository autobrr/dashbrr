/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import {
  ArrowTopRightOnSquareIcon,
  Cog6ToothIcon,
  TrashIcon
} from "@heroicons/react/20/solid";
import React, { useState } from "react";
import { repoUrls } from "../../config/repoUrls";
import { ServiceStatus } from "../../types/service";
import AnimatedModal from "./AnimatedModal";
import { StatusIcon, StatusType } from "./StatusIcon";

interface ServiceHeaderProps {
  displayName: string;
  url?: string;
  accessUrl?: string;
  version?: string;
  updateAvailable?: boolean;
  healthEndpoint?: string;
  onConfigure: (e?: React.MouseEvent) => void;
  onRemove: (e?: React.MouseEvent) => void;
  needsConfiguration?: boolean;
  status?: ServiceStatus;
}

export const ServiceHeader: React.FC<ServiceHeaderProps> = ({
  displayName,
  url,
  accessUrl,
  version,
  updateAvailable,
  onConfigure,
  onRemove,
  needsConfiguration,
  status,
}) => {
  const [showRemoveModal, setShowRemoveModal] = useState(false);


  const trimVersion = (version: string): string => {
    if (!version) return "Unknown";
    if (version.startsWith("pr-")) return version;
    const versionParts = version.split("-")[0].split(".");
    return versionParts.slice(0, 3).join(".");
  };

  const handleRemoveClick = (e?: React.MouseEvent) => {
    e?.stopPropagation();
    setShowRemoveModal(true);
  };

  const handleConfirmRemove = () => {
    onRemove();
    setShowRemoveModal(false);
  };

  const getUpdateUrl = () => {
    const serviceKey = displayName.toLowerCase();
    return repoUrls[serviceKey];
  };

  // Use accessUrl if available, otherwise fall back to url
  const openUrl = accessUrl || url;

  const serviceKey = displayName.toLowerCase().replace(/\s+/g, "-");

  return (
    <>
      <div className="flex items-start justify-between gap-2 @md:items-center">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 overflow-hidden">
            <h3 className="flex items-center gap-2 truncate text-base font-semibold text-zinc-900 dark:text-white @md:text-lg">
              <a
                href={openUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-inherit hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-200"
                onClick={(e) => e.stopPropagation()}
              >
                {/* Light mode icon */}
                <img
                  key={`${serviceKey}-light`}
                  src={`/icons/${serviceKey}-light.svg`}
                  alt=""
                  className="w-5 h-5 flex-shrink-0 object-contain dark:hidden"
                  onError={(e) => {
                    if (!e.currentTarget.src.endsWith(`/icons/${serviceKey}.svg`)) {
                      e.currentTarget.src = `/icons/${serviceKey}.svg`;
                    } else {
                      e.currentTarget.style.display = "none";
                    }
                  }}
                />
                {/* Dark mode icon */}
                <img
                  key={`${serviceKey}-dark`}
                  src={`/icons/${serviceKey}-dark.svg`}
                  alt=""
                  className="w-5 h-5 flex-shrink-0 object-contain hidden dark:block"
                  onError={(e) => {
                    if (!e.currentTarget.src.endsWith(`/icons/${serviceKey}.svg`)) {
                      e.currentTarget.src = `/icons/${serviceKey}.svg`;
                    } else {
                      e.currentTarget.style.display = "none";
                    }
                  }}
                />
                {displayName}
                {openUrl && (
                  <ArrowTopRightOnSquareIcon className="w-4 h-4 text-blue-600 dark:text-blue-400 flex-shrink-0 transition-transform duration-200 hover:scale-110" />
                )}
              </a>
            </h3>
            {version && (
              <span
                className={`inline-flex items-center justify-center px-2 py-1 rounded text-xs font-medium transition-colors duration-200 flex-shrink-0 ${
                  updateAvailable
                    ? "text-green-600 dark:text-green-400 bg-green-50/90 dark:bg-green-900/30"
                    : "bg-zinc-100 dark:bg-zinc-800 text-zinc-800 dark:text-zinc-200"
                }`}
              >
                {updateAvailable ? (
                  <a
                    href={getUpdateUrl()}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="hover:text-green-600 dark:hover:text-green-200 transition-colors duration-200"
                    onClick={(e) => e.stopPropagation()}
                  >
                    Update
                  </a>
                ) : (
                  trimVersion(version)
                )}
              </span>
            )}
          </div>
        </div>
        <div className="ml-2 flex items-center space-x-2 @md:ml-4">
          <div
            className={`flex items-center ${
              needsConfiguration
                ? ""
                : "opacity-100 @md:opacity-0 @md:group-hover:opacity-100"
            } transition-all duration-200`}
          >
            <button
              onClick={(e) => {
                e.stopPropagation();
                onConfigure(e);
              }}
              className={`p-1.5 rounded-full transition-all duration-200 ${
                needsConfiguration
                  ? "text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-500/20"
                  : "text-zinc-400 hover:text-zinc-600 dark:hover:text-white hover:bg-zinc-100 dark:hover:bg-zinc-700"
              }`}
              title="Configure service"
            >
              <Cog6ToothIcon className="h-4 w-4" />
            </button>
            <button
              onClick={handleRemoveClick}
              className="rounded-full p-1.5 text-red-500 transition-all duration-200 hover:bg-red-50 hover:text-red-700 dark:text-red-400 dark:hover:bg-red-500/20 dark:hover:text-red-300"
              title="Remove service"
            >
              <TrashIcon className="h-4 w-4" />
            </button>
          </div>
          {status && (
            <div className="flex-shrink-0">
              <StatusIcon status={status as StatusType} />
            </div>
          )}
        </div>
      </div>

      <AnimatedModal
        isOpen={showRemoveModal}
        onClose={() => setShowRemoveModal(false)}
        title="Remove Service"
        maxWidth="sm"
      >
        <div className="mt-2">
          <p className="text-zinc-600 dark:text-zinc-300">
            Are you sure you want to remove {displayName}? This action cannot be
            undone.
          </p>
        </div>

        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            className="inline-flex justify-center rounded-md border border-zinc-300 dark:border-zinc-600 px-4 py-2 text-sm font-medium text-zinc-700 dark:text-zinc-300 hover:bg-zinc-50 dark:hover:bg-zinc-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 transition-colors duration-200"
            onClick={() => setShowRemoveModal(false)}
          >
            Cancel
          </button>
          <button
            type="button"
            className="inline-flex justify-center rounded-md border border-transparent bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500 focus-visible:ring-offset-2 transition-colors duration-200"
            onClick={handleConfirmRemove}
          >
            Remove
          </button>
        </div>
      </AnimatedModal>
    </>
  );
};
