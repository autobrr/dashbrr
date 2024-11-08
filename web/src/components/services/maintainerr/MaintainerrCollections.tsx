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
      <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4">
        Loading collections...
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
