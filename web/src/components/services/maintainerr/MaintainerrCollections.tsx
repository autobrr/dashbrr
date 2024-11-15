/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { ArrowTopRightOnSquareIcon } from "@heroicons/react/24/solid";

interface Props {
  instanceId: string;
}

export const MaintainerrCollections: React.FC<Props> = ({ instanceId }) => {
  const { services } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);
  const collections = service?.stats?.maintainerr?.collections || [];
  const isLoading = !service || service.status === "loading";

  if (isLoading) {
    return (
      <div className="space-y-3">
        {[1, 2, 3].map((i) => (
          <div
            key={i}
            className="flex items-center space-x-3 bg-gray-50 dark:bg-gray-700/50 p-3 rounded-lg animate-pulse"
          >
            <div className="min-w-0 flex-1">
              <div className="h-4 bg-gray-200 dark:bg-gray-600 rounded w-3/4 mb-2" />
              <div className="flex space-x-2">
                <div className="h-3 bg-gray-200 dark:bg-gray-600 rounded w-20" />
                <div className="h-3 bg-gray-200 dark:bg-gray600 rounded w-24" />
              </div>
            </div>
            <div className="flex-shrink-0">
              <div className="h-4 bg-gray-200 dark:bg-gray-600 rounded w-16" />
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (collections.length === 0) {
    return null;
  }

  return (
    <>
      <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300">
        Collections:
      </div>
      {collections.map((collection) => (
        <div key={collection.id} className="mt-2">
          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-1">
            <div>
              <span className="font-medium text-xs text-gray-600 dark:text-gray-300">
                <a
                  href={`${service?.url}/collections`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-blue-600 dark:text-blue-400 flex items-center"
                >
                  {collection.title}
                  <ArrowTopRightOnSquareIcon className="ml-1 w-3 h-3 text-blue-400" />
                </a>
              </span>
            </div>
            <div>
              <span className="text-xs text-gray-600 dark:text-gray-300">
                Delete after:{" "}
              </span>
              {collection.deleteAfterDays} days
            </div>
            <div>
              <span className="text-xs text-gray-600 dark:text-gray-300">
                Media items:{" "}
              </span>
              {collection.media.length}
            </div>
          </div>
        </div>
      ))}
    </>
  );
};
