/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useState } from "react";
import { useServiceData } from "../../../hooks/useServiceData";
import { RadarrQueueItem } from "../../../types/service";
import { RadarrMessage } from "./RadarrMessage";
import AnimatedModal from "../../ui/AnimatedModal";
import {
  Cog6ToothIcon,
  ArrowTopRightOnSquareIcon,
  ChevronDownIcon,
} from "@heroicons/react/24/solid";
import { api } from "../../../utils/api";
import Toast from "../../../components/Toast";
import { toast } from "react-hot-toast";
import { Listbox } from "@headlessui/react";
import { Transition } from "@headlessui/react";
import { Fragment } from "react";
import { StatsSkeleton } from "../../ui/StatsSkeleton";

interface RadarrStatsProps {
  instanceId: string;
}

interface DeleteOptions {
  removeFromClient: "remove" | "change" | "ignore";
  blocklist: "none" | "blocklist" | "blocklistAndSearch";
}

// Helper function to get display text for removal method
const getRemovalMethodText = (value: DeleteOptions["removeFromClient"]) => {
  switch (value) {
    case "remove":
      return "Remove from Download Client";
    case "change":
      return "Change Category";
    case "ignore":
      return "Ignore Download";
  }
};

// Helper function to get display text for blocklist
const getBlocklistText = (value: DeleteOptions["blocklist"]) => {
  switch (value) {
    case "none":
      return "Do not Blocklist";
    case "blocklistAndSearch":
      return "Blocklist and Search";
    case "blocklist":
      return "Blocklist Only";
  }
};

export const RadarrStats: React.FC<RadarrStatsProps> = ({ instanceId }) => {
  const { services } = useServiceData();
  const service = services.find((s) => s.instanceId === instanceId);
  const isLoading = service?.status === "loading";

  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [selectedItem, setSelectedItem] = useState<RadarrQueueItem | null>(
    null
  );
  const [deleteOptions, setDeleteOptions] = useState<DeleteOptions>({
    removeFromClient: "change",
    blocklist: "none",
  });

  const handleDelete = async () => {
    if (!selectedItem) return;

    try {
      const queryParams = new URLSearchParams();
      queryParams.append("instanceId", instanceId);

      // Handle removeFromClient and changeCategory based on selected option
      if (deleteOptions.removeFromClient === "change") {
        queryParams.append("removeFromClient", "false");
        queryParams.append("changeCategory", "true");
      } else if (deleteOptions.removeFromClient === "remove") {
        queryParams.append("removeFromClient", "true");
        queryParams.append("changeCategory", "false");
      } else {
        // ignore
        queryParams.append("removeFromClient", "false");
        queryParams.append("changeCategory", "false");
      }

      // Handle blocklist options
      if (deleteOptions.blocklist === "blocklistAndSearch") {
        queryParams.append("blocklist", "true");
        queryParams.append("skipRedownload", "false");
      } else if (deleteOptions.blocklist === "blocklist") {
        queryParams.append("blocklist", "true");
        queryParams.append("skipRedownload", "true");
      } else {
        // none
        queryParams.append("blocklist", "false");
        queryParams.append("skipRedownload", "true");
      }

      await api.delete(
        `/api/radarr/queue/${selectedItem.id}?${queryParams.toString()}`
      );

      if (service?.stats?.radarr?.queue) {
        service.stats.radarr.queue.records =
          service.stats.radarr.queue.records.filter(
            (item) => item.id !== selectedItem.id
          );
        service.stats.radarr.queue.totalRecords--;
      }

      setShowDeleteModal(false);
      setSelectedItem(null);
      toast.custom((t) => (
        <Toast type="success" body="Successfully removed from queue" t={t} />
      ));
    } catch (error) {
      console.error("Failed to delete queue item:", error);
      toast.custom((t) => (
        <Toast type="error" body="Failed to remove from queue" t={t} />
      ));
    }
  };

  if (isLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  // Use accessUrl if available, otherwise fall back to url
  const openUrl = service.accessUrl || service.url;

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <RadarrMessage status={service.status} message={service.message} />

      {/* Queue Display */}
      {service.stats?.radarr?.queue &&
        service.stats.radarr.queue.totalRecords > 0 && (
          <div>
            <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300 cursor-default">
              Queue ({service.stats.radarr.queue.totalRecords}):
            </div>
            <div className="space-y-2">
              {service.stats.radarr.queue.records
                .slice(0, 3)
                .map((record: RadarrQueueItem) => (
                  <div
                    key={record.id}
                    className="flex flex-col p-3 text-sm rounded-lg bg-gray-850/95 dark:bg-gray-850/95 "
                  >
                    <div className="flex items-center justify-between w-full">
                      <div className="min-w-0 flex-1 mr-2">
                        <span
                          className="block truncate text-xs font-medium text-gray-300 cursor-default"
                          title={record.title}
                        >
                          {record.title}
                        </span>
                      </div>
                      <div className="flex-shrink-0 flex items-center space-x-1">
                        <a
                          href={`${openUrl}/activity/queue`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="p-1.5 rounded-md hover:bg-gray-700 dark:hover:bg-gray-700 transition-colors"
                          title="View in Radarr"
                        >
                          <ArrowTopRightOnSquareIcon className="h-4 w-4 text-gray-400" />
                        </a>
                        <button
                          onClick={() => {
                            setSelectedItem(record);
                            setShowDeleteModal(true);
                          }}
                          disabled={record.trackedDownloadState !== "importBlocked" && record.trackedDownloadState !== "importPending"}
                          className={`p-1.5 rounded-md transition-colors ${
                            record.trackedDownloadState === "importBlocked"
                              ? "hover:bg-gray-700 dark:hover:bg-gray-700"
                              : "opacity-50 cursor-not-allowed"
                          }`}
                          title={
                            record.trackedDownloadState === "importBlocked"
                              ? "Manage queue"
                              : "Can only remove items that are import blocked"
                          }
                        >
                          <Cog6ToothIcon className="h-4 w-4 text-gray-400" />
                        </button>
                      </div>
                    </div>
                    <div className="mt-1.5 flex flex-wrap gap-2 text-xs text-gray-400 pointer-events-none">
                      <div className="flex items-center gap-1">
                        <span className="font-medium text-gray-500">State:</span>
                        <span>{record.trackedDownloadState}</span>
                      </div>
                      {record.indexer && (
                        <div className="flex items-center gap-1">
                          <span className="font-medium text-gray-500">Indexer:</span>
                          <span>{record.indexer}</span>
                        </div>
                      )}
                      {record.customFormatScore != null && (
                        <div className="flex items-center gap-1">
                          <span className="font-medium text-gray-500">CF Score:</span>
                          <span>{record.customFormatScore}</span>
                        </div>
                      )}
                      <div className="flex items-center gap-1">
                        <span className="font-medium text-gray-500">Client:</span>
                        <span>{record.downloadClient}</span>
                      </div>
                      <div className="flex items-center gap-1">
                        <span className="font-medium text-gray-500">Protocol:</span>
                        <span className="capitalize">{record.protocol}</span>
                      </div>
                    </div>
                    {record.statusMessages &&
                      record.statusMessages.length > 0 && (
                        <div className="mt-2 w-full space-y-1 pointer-events-none">
                          {record.statusMessages.map((msg, idx) => (
                            <div key={idx} className="space-y-1 w-full">
                              {msg.messages &&
                                msg.messages.map((message, msgIdx) => (
                                  <div
                                    key={msgIdx}
                                    className="text-xs p-2 w-full rounded-lg text-amber-300 bg-amber-900/20 border border-amber-800/40"
                                  >
                                    {message}
                                  </div>
                                ))}
                            </div>
                          ))}
                        </div>
                      )}
                  </div>
                ))}
            </div>
          </div>
        )}

      {/* Delete Confirmation Modal */}
      <AnimatedModal
        isOpen={showDeleteModal}
        onClose={() => {
          setShowDeleteModal(false);
          setSelectedItem(null);
        }}
        title="Manage Download"
        className="min-h-[400px] max-h-[90vh]"
      >
        <div className="space-y-4">
          <p className="text-md font-medium text-gray-600 dark:text-gray-400">
            Are you sure you want to remove this release from the queue?
          </p>
          <p className="text-xs">
            <a
              href={`${openUrl}/activity/queue`}
              target="_blank"
              rel="noopener noreferrer"
              className="font-bold px-2 py-1 rounded-md bg-gray-750 text-gray-700 dark:text-gray-300 break-all inline-block hover:bg-gray-700 transition-colors"
            >
              {selectedItem?.title}
            </a>
          </p>

          {selectedItem?.statusMessages &&
            selectedItem.statusMessages.length > 0 && (
              <div className="space-y-1">
                {selectedItem.statusMessages.map((msg, idx) => (
                  <div key={idx} className="text-xs break-all text-amber-300">
                    {msg.messages &&
                      msg.messages.map((message, msgIdx) => (
                        <p key={msgIdx}>{message}</p>
                      ))}
                  </div>
                ))}
              </div>
            )}

          <div className="space-y-4">
            <div className="space-y-2 max-w-full">
              <div className="flex flex-col space-y-1">
                <label className="text-xs text-gray-700 dark:text-gray-300">
                  Removal Method
                </label>
                <Listbox
                  value={deleteOptions.removeFromClient}
                  onChange={(value) =>
                    setDeleteOptions({
                      ...deleteOptions,
                      removeFromClient:
                        value as DeleteOptions["removeFromClient"],
                    })
                  }
                >
                  {({ open }) => (
                    <div className="relative">
                      <Listbox.Button className="relative w-full rounded-md bg-gray-700 py-2 pl-3 pr-10 text-left text-gray-300 shadow-sm sm:text-xs">
                        <span className="block truncate">
                          {getRemovalMethodText(deleteOptions.removeFromClient)}
                        </span>
                        <span className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-4">
                          <ChevronDownIcon
                            className={`h-4 w-4 text-gray-400 transition-transform duration-200 ${
                              open ? "transform rotate-180" : ""
                            }`}
                            aria-hidden="true"
                          />
                        </span>
                      </Listbox.Button>
                      <Transition
                        as={Fragment}
                        leave="transition ease-in duration-100"
                        leaveFrom="opacity-100"
                        leaveTo="opacity-0"
                      >
                        <Listbox.Options className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md bg-gray-700 py-1 shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none sm:text-xs">
                          <Listbox.Option
                            value="remove"
                            className={({ active }) =>
                              `relative cursor-pointer select-none py-2 pl-3 pr-9 transition-colors ${
                                active
                                  ? "bg-gray-600 text-gray-200"
                                  : "text-gray-300"
                              }`
                            }
                          >
                            Remove from Download Client
                          </Listbox.Option>
                          {selectedItem?.protocol !== "usenet" && (
                            <Listbox.Option
                              value="change"
                              className={({ active }) =>
                                `relative cursor-pointer select-none py-2 pl-3 pr-9 transition-colors ${
                                  active
                                    ? "bg-gray-600 text-gray-200"
                                    : "text-gray-300"
                                }`
                              }
                            >
                              Change Category
                            </Listbox.Option>
                          )}
                          <Listbox.Option
                            value="ignore"
                            className={({ active }) =>
                              `relative cursor-pointer select-none py-2 pl-3 pr-9 transition-colors ${
                                active
                                  ? "bg-gray-600 text-gray-200"
                                  : "text-gray-300"
                              }`
                            }
                          >
                            Ignore Download
                          </Listbox.Option>
                        </Listbox.Options>
                      </Transition>
                    </div>
                  )}
                </Listbox>
              </div>

              <div className="flex flex-col space-y-1">
                <label className="text-xs text-gray-700 dark:text-gray-300">
                  Blocklist Release
                </label>
                <Listbox
                  value={deleteOptions.blocklist}
                  onChange={(value) =>
                    setDeleteOptions({
                      ...deleteOptions,
                      blocklist: value as DeleteOptions["blocklist"],
                    })
                  }
                >
                  {({ open }) => (
                    <div className="relative">
                      <Listbox.Button className="relative w-full rounded-md bg-gray-700 py-2 pl-3 pr-10 text-left text-gray-300 shadow-sm sm:text-xs">
                        <span className="block truncate">
                          {getBlocklistText(deleteOptions.blocklist)}
                        </span>
                        <span className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-4">
                          <ChevronDownIcon
                            className={`h-4 w-4 text-gray-400 transition-transform duration-200 ${
                              open ? "transform rotate-180" : ""
                            }`}
                            aria-hidden="true"
                          />
                        </span>
                      </Listbox.Button>
                      <Transition
                        as={Fragment}
                        leave="transition ease-in duration-100"
                        leaveFrom="opacity-100"
                        leaveTo="opacity-0"
                      >
                        <Listbox.Options className="absolute z-10 bottom-full mb-1 max-h-60 w-full overflow-auto rounded-md bg-gray-700 py-1 shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none sm:text-xs">
                          <Listbox.Option
                            value="none"
                            className={({ active }) =>
                              `relative cursor-pointer select-none py-2 pl-3 pr-9 transition-colors ${
                                active
                                  ? "bg-gray-600 text-gray-200"
                                  : "text-gray-300"
                              }`
                            }
                          >
                            Do not Blocklist
                          </Listbox.Option>
                          <Listbox.Option
                            value="blocklistAndSearch"
                            className={({ active }) =>
                              `relative cursor-pointer select-none py-2 pl-3 pr-9 transition-colors ${
                                active
                                  ? "bg-gray-600 text-gray-200"
                                  : "text-gray-300"
                              }`
                            }
                          >
                            Blocklist and Search
                          </Listbox.Option>
                          <Listbox.Option
                            value="blocklist"
                            className={({ active }) =>
                              `relative cursor-pointer select-none py-2 pl-3 pr-9 transition-colors ${
                                active
                                  ? "bg-gray-600 text-gray-200"
                                  : "text-gray-300"
                              }`
                            }
                          >
                            Blocklist Only
                          </Listbox.Option>
                        </Listbox.Options>
                      </Transition>
                    </div>
                  )}
                </Listbox>
              </div>
            </div>
          </div>

          <div className="flex justify-end space-x-3 mt-6">
            <button
              onClick={() => {
                setShowDeleteModal(false);
                setSelectedItem(null);
              }}
              className="px-4 py-2 text-xs font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700/50 rounded-md transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleDelete}
              className="px-4 py-2 text-xs font-medium text-white bg-red-600 hover:bg-red-700 rounded-md transition-colors"
            >
              Remove
            </button>
          </div>
        </div>
      </AnimatedModal>
    </div>
  );
};
