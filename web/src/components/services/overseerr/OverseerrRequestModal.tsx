/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import AnimatedModal from "../../ui/AnimatedModal";
import { OverseerrMediaRequest } from "../../../types/service";

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

  return (
    <AnimatedModal
      isOpen={isOpen}
      onClose={onClose}
      title={`Confirm ${actionText} request`}
      maxWidth="md"
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-700 dark:text-gray-300">
          Are you sure you want to {actionText} the request for:
        </p>
        <div className="bg-gray-100 dark:bg-gray-700/50 rounded-lg p-4">
          <h3 className="font-medium text-gray-900 dark:text-white">
            {request.media.title ||
              `${request.media.mediaType === "tv" ? "TV Show" : "Movie"} (${
                request.media.tmdbId
              })`}
          </h3>
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Requested by:{" "}
            {request.requestedBy.username || request.requestedBy.email}
          </p>
        </div>
        <div className="flex justify-end gap-3 pt-4">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-800 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => {
              onConfirm();
              onClose();
            }}
            className={`px-4 py-2 text-sm font-medium text-white bg-${actionColor}-600 rounded-lg hover:bg-${actionColor}-700 transition-colors`}
          >
            {actionText.charAt(0).toUpperCase() + actionText.slice(1)}
          </button>
        </div>
      </div>
    </AnimatedModal>
  );
};
