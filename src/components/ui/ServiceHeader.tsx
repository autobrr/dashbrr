import React, { useState } from "react";
import {
  ArrowTopRightOnSquareIcon,
  Cog6ToothIcon,
  TrashIcon,
} from "@heroicons/react/20/solid";
import AnimatedModal from "./AnimatedModal";
import { StatusIcon } from "./StatusIcon";
import { repoUrls } from "../../config/repoUrls";

type StatusType =
  | "online"
  | "offline"
  | "warning"
  | "error"
  | "loading"
  | "unknown";

interface ServiceHeaderProps {
  displayName: string;
  url?: string;
  version?: string;
  updateAvailable?: boolean;
  healthEndpoint?: string;
  onConfigure: (e?: React.MouseEvent) => void;
  onRemove: (e?: React.MouseEvent) => void;
  needsConfiguration?: boolean;
  status?: StatusType;
}

export const ServiceHeader: React.FC<ServiceHeaderProps> = ({
  displayName,
  url,
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

  return (
    <>
      <div className="flex justify-between items-center">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 overflow-hidden">
            <h3 className="flex items-center gap-2 text-lg font-semibold text-gray-900 dark:text-white truncate">
              <a
                href={url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-inherit hover:text-blue-600 dark:hover:text-blue-400 transition-colors duration-200"
                onClick={(e) => e.stopPropagation()}
              >
                {displayName}
                {url && (
                  <ArrowTopRightOnSquareIcon className="w-4 h-4 text-blue-600 dark:text-blue-400 flex-shrink-0 transition-transform duration-200 hover:scale-110" />
                )}
              </a>
            </h3>
            {version && (
              <span
                className={`inline-flex items-center justify-center px-2 py-1 rounded text-xs font-medium transition-colors duration-200 flex-shrink-0 ${
                  updateAvailable
                    ? "bg-yellow-100 dark:bg-yellow-900/50 text-yellow-800 dark:text-yellow-100"
                    : "bg-gray-100 dark:bg-gray-800 text-gray-800 dark:text-gray-200"
                }`}
              >
                {updateAvailable ? (
                  <a
                    href={getUpdateUrl()}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="hover:text-yellow-600 dark:hover:text-yellow-300 transition-colors duration-200"
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
        <div className="flex items-center space-x-2 ml-4">
          <div
            className={`flex items-center ${
              needsConfiguration ? "" : "opacity-0 group-hover:opacity-100"
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
                  : "text-gray-400 hover:text-gray-600 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-gray-700"
              }`}
              title="Configure service"
            >
              <Cog6ToothIcon className="h-4 w-4" />
            </button>
            <button
              onClick={handleRemoveClick}
              className="p-1.5 rounded-full text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-500/20 transition-all duration-200"
              title="Remove service"
            >
              <TrashIcon className="h-4 w-4" />
            </button>
          </div>
          {status && (
            <div className="flex-shrink-0">
              <StatusIcon status={status} />
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
          <p className="text-gray-600 dark:text-gray-300">
            Are you sure you want to remove {displayName}? This action cannot be
            undone.
          </p>
        </div>

        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            className="inline-flex justify-center rounded-md border border-gray-300 dark:border-gray-600 px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 transition-colors duration-200"
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
