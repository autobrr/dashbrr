/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { ArrowTopRightOnSquareIcon, ClockIcon, FilmIcon } from "@heroicons/react/24/outline";
import { StatsSkeleton } from "../../ui/StatsSkeleton";

interface Props {
  instanceId: string;
}

export const MaintainerrCollections: React.FC<Props> = ({ instanceId }) => {
  const { getService } = useServiceData();
  const service = getService(instanceId);
  const collections = service?.stats?.maintainerr?.collections || [];
  const isLoading = !service || service.status === "loading";

  if (isLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (collections.length === 0) {
    return null;
  }

  return (
    <>
      <div className="text-xs mb-2 pt-2 font-semibold text-gray-700 dark:text-gray-300 cursor-default">
        Collections:
      </div>
      {collections.map((collection) => (
        <div key={collection.id} className="mt-2">
          <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-3.5  transition-colors">
            <div>
              <span className="font-medium text-gray-200 truncate">
                <a
                  href={`${service?.accessUrl || service?.url}/collections`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-white hover:text-blue-400 flex items-center group"
                >
                  {collection.title}
                  <ArrowTopRightOnSquareIcon className="ml-1 w-3 h-3 text-blue-400 group-hover:text-blue-400" />
                </a>
              </span>
            </div>
            <div className="flex items-center gap-4 mt-1">
              <div className="flex items-center gap-1">
                <ClockIcon className="w-3.5 h-3.5 text-gray-400" />
                <span className="text-gray-600 dark:text-gray-400">Delete after:</span>
                <span className="text-gray-700 dark:text-gray-200">{collection.deleteAfterDays} days</span>
              </div>
              <div className="flex items-center gap-1">
                <FilmIcon className="w-3.5 h-3.5 text-gray-400" />
                <span className="text-gray-700 dark:text-gray-200">{collection.media.length}</span>
              </div>
            </div>
          </div>
        </div>
      ))}
    </>
  );
};
