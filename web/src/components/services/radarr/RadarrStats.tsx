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
                <div className="h-3 bg-gray-200 dark:bg-gray-600 rounded w-24" />
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

  if (!service) {
    return null;
  }

  return (
    <div className="space-y-4">
      {/* Status and Messages */}
      <RadarrMessage status={service.status} message={service.message} />

      {/* Queue Display */}
      {service.stats?.radarr?.queue &&
        service.stats.radarr.queue.totalRecords > 0 && (
          <div>
            <div className="text-xs pb-2 font-semibold text-gray-700 dark:text-gray-300">
              Queue ({service.stats.radarr.queue.totalRecords}):
            </div>
            <div className="text-xs rounded-md text-gray-600 dark:text-gray-400 bg-gray-850/95 p-4 space-y-2">
              {service.stats.radarr.queue.records
                .slice(0, 3)
                .map(
                  (
                    record: RadarrQueueItem,
                    index: number,
                    array: RadarrQueueItem[]
                  ) => (
                    <div
                      key={record.id}
                      className={`flex flex-col space-y-1 overflow-hidden px-0 ${
                        index !== array.length - 1
                          ? "border-b border-gray-750 pb-2"
                          : ""
                      }`}
                    >
                      <div className="flex justify-between items-center w-full">
                        <div className="flex-1 min-w-0">
                          <div className="text-xs opacity-75 flex items-center justify-between w-full">
                            <div className="flex items-center min-w-0 flex-1">
                              <span className="flex-shrink-0 font-medium text-xs text-gray-600 dark:text-gray-300">
                                Release:{" "}
                              </span>
                              <span
                                className="truncate text-xs ml-1 cursor-help"
                                title={record.title}
                              >
                                {record.title}
                              </span>
                            </div>
                            <div className="flex space-x-2 flex-shrink-0 ml-2">
                              <a
                                href={`${service.url}/activity/queue`}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="p-1 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700/50 transition-colors"
                                title="View in Radarr"
                              >
                                <ArrowTopRightOnSquareIcon className="h-4 text-gray-500 dark:text-gray-400" />
                              </a>
                              <button
                                onClick={() => {
                                  setSelectedItem(record);
                                  setShowDeleteModal(true);
                                }}
                                disabled={
                                  record.trackedDownloadState !==
                                  "importBlocked"
                                }
                                className={`p-1 rounded-md transition-colors ${
                                  record.trackedDownloadState ===
                                  "importBlocked"
                                    ? "hover:bg-gray-100 dark:hover:bg-gray-700/50"
                                    : "opacity-50 cursor-not-allowed"
                                }`}
                                title={
                                  record.trackedDownloadState ===
                                  "importBlocked"
                                    ? "Manage queue"
                                    : "Can only remove items that are import blocked"
                                }
                              >
                                <Cog6ToothIcon className="h-4 text-gray-500 dark:text-gray-400" />
                              </button>
                            </div>
                          </div>
                          <div className="text-xs opacity-75">
                            <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                              State:{" "}
                            </span>
                            {record.trackedDownloadState}
                          </div>
                          {record.indexer && (
                            <div className="text-xs opacity-75">
                              <span className="truncate flex-1 font-medium text-xs text-gray-600 dark:text-gray-300">
                                Indexer:{" "}
                              </span>
                              {record.indexer}
                            </div>
                          )}
                          {record.customFormatScore != null && (
                            <div className="text-xs opacity-75">
                              <span className="font-medium text-xs text-gray-600 dark:text-gray-300">
                                Custom Format Score:{" "}
                              </span>
                              {record.customFormatScore}
                            </div>
                          )}
                          <div className="text-xs opacity-75">
                            <span className="font-medium text-xs text-gray-600 dark:text-gray-300">
                              Client:{" "}
                            </span>
                            {record.downloadClient}
                          </div>
                          <div className="text-xs opacity-75">
                            <span className="font-medium text-xs text-gray-600 dark:text-gray-300">
                              Protocol:{" "}
                            </span>
                            {record.protocol}
                          </div>
                          {record.statusMessages &&
                            record.statusMessages.length > 0 && (
                              <div className="mt-2 space-y-1">
                                {record.statusMessages.map((msg, idx) => (
                                  <div
                                    key={idx}
                                    className="flex items-start space-x-1 text-amber-300"
                                  >
                                    <div className="flex-1 min-w-0 space-y-1">
                                      {msg.messages &&
                                        msg.messages.map((message, msgIdx) => {
                                          const [firstPart, ...rest] =
                                            message.split(".");
                                          return (
                                            <div
                                              key={msgIdx}
                                              className="text-xs space-y-1 p-2 rounded-lg transition-all duration-200 backdrop-blur-sm text-amber-500 dark:text-amber-300 bg-amber-50/90 dark:bg-amber-900/20 border border-amber-100 dark:border-amber-800/40"
                                            >
                                              <div className="font-normal">
                                                {firstPart}.
                                              </div>
                                              {rest.length > 0 && (
                                                <div className="">
                                                  {rest.join(".")}
                                                </div>
                                              )}
                                            </div>
                                          );
                                        })}
                                    </div>
                                  </div>
                                ))}
                              </div>
                            )}
                        </div>
                      </div>
                    </div>
                  )
                )}
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
              href={`${service.url}/activity/queue`}
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
