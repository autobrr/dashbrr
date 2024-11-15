/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import AnimatedModal from "../../ui/AnimatedModal";
import { OverseerrMediaRequest } from "../../../types/service";
import { FaFilm, FaTv } from "react-icons/fa";
import { ClockIcon, UserIcon } from "@heroicons/react/24/outline";

interface OverseerrRequestModalProps {
  isOpen: boolean;
  onClose: () => void;
  request: OverseerrMediaRequest;
  onConfirm: () => void;
  action: "approve" | "reject";
}

export const OverseerrRequestModal: React.FC<OverseerrRequestModalProps> = ({
  isOpen,
  onClose,
  request,
  onConfirm,
  action,
}) => {
  const actionText = action === "approve" ? "approve" : "reject";
  const actionColor = action === "approve" ? "green" : "red";

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  };

  const getUserDisplayName = (
    requestedBy: OverseerrMediaRequest["requestedBy"]
  ) => {
    if (!requestedBy) return "Unknown User";
    return (
      requestedBy.username ||
      requestedBy.plexUsername ||
      requestedBy.email ||
      "Unknown User"
    );
  };

  return (
    <AnimatedModal
      isOpen={isOpen}
      onClose={onClose}
      title={`${
        actionText.charAt(0).toUpperCase() + actionText.slice(1)
      } request`}
      maxWidth="md"
    >
      <div className="space-y-6">
        <div className="bg-gradient-to-b from-gray-800/80 to-gray-800/40 rounded-xl border border-gray-700/50 shadow-lg backdrop-blur-sm overflow-hidden">
          <div className="p-6 relative">
            <div
              className={`absolute inset-0 bg-gradient-to-br from-${actionColor}-500/5 to-transparent pointer-events-none`}
            />

            <div className="flex items-start gap-4 relative">
              <div className="space-y-4 flex-1">
                <div>
                  <div className="flex items-center gap-3">
                    <div
                      className={`p-2 rounded-lg bg-gray-700/50 border border-gray-600/50 shadow-inner h-fit`}
                    >
                      {request.media.tvdbId ? (
                        <FaTv className="h-5 w-5 text-blue-400" />
                      ) : (
                        <FaFilm className="h-5 w-5 text-purple-400" />
                      )}
                    </div>
                    <h3 className="text-2xl font-semibold text-white tracking-tight">
                      {request.media.title ||
                        `${
                          request.media.mediaType === "tv" ? "TV Show" : "Movie"
                        } (${request.media.tmdbId})`}
                    </h3>
                  </div>
                  <div className="flex items-center gap-2 mt-1.5 flex-wrap">
                    {request.media.tmdbId && (
                      <span className="px-3 py-0.5 rounded-md bg-gray-700/50 border border-gray-600/50 text-xs font-medium text-gray-300 min-w-[120px] text-center">
                        TMDB: {request.media.tmdbId}
                      </span>
                    )}
                    {request.media.tvdbId && (
                      <span className="px-3 py-0.5 rounded-md bg-gray-700/50 border border-gray-600/50 text-xs font-medium text-gray-300 min-w-[120px] text-center">
                        TVDB: {request.media.tvdbId}
                      </span>
                    )}
                  </div>
                </div>

                <div className="space-y-2 text-sm">
                  <div className="flex items-center text-gray-300 gap-2">
                    <UserIcon className="h-4 w-4 text-gray-400" />
                    <span className="font-medium">Requested by:</span>
                    <span className="text-gray-400">
                      {getUserDisplayName(request.requestedBy)}
                    </span>
                  </div>
                  <div className="flex items-center text-gray-300 gap-2">
                    <ClockIcon className="h-4 w-4 text-gray-400" />
                    <span className="font-medium">Date requested:</span>
                    <span className="text-gray-400">
                      {formatDate(request.createdAt)}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="px-1">
          <p className="text-base text-gray-300">
            Are you sure you want to {actionText} this request?
          </p>
        </div>

        <div className="flex justify-end gap-3 pt-2">
          <button
            onClick={onClose}
            className="px-5 py-2.5 text-sm font-medium text-gray-300 bg-gray-800 rounded-lg border border-gray-700 hover:bg-gray-700 hover:border-gray-600 transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-gray-600 focus:ring-offset-2 focus:ring-offset-gray-800"
          >
            Cancel
          </button>
          <button
            onClick={() => {
              onConfirm();
              onClose();
            }}
            className={`px-5 py-2.5 text-sm font-medium text-white bg-${actionColor}-600 rounded-lg border border-${actionColor}-500 hover:bg-${actionColor}-500 transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-${actionColor}-500 focus:ring-offset-2 focus:ring-offset-gray-800 shadow-sm shadow-${actionColor}-500/20`}
          >
            {actionText.charAt(0).toUpperCase() + actionText.slice(1)}
          </button>
        </div>
      </div>
    </AnimatedModal>
  );
};
