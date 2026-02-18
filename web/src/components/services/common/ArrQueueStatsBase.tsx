/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { Fragment, useState } from "react";
import { Listbox, Transition } from "@headlessui/react";
import {
  ArrowTopRightOnSquareIcon,
  ChevronDownIcon,
  Cog6ToothIcon,
} from "@heroicons/react/24/solid";
import { toast } from "react-hot-toast";

import { useServiceData } from "../../../hooks/useServiceData";
import { ServiceStats, ServiceStatus } from "../../../types/service";
import { api } from "../../../utils/api";
import Toast from "../../../components/Toast";
import AnimatedModal from "../../ui/AnimatedModal";
import { StatsSkeleton } from "../../ui/StatsSkeleton";
import {
  ArrQueueDeleteOptions,
  buildArrQueueDeleteQueryParams,
  getBlocklistText,
  getRemovalMethodText,
} from "./ArrQueueDelete";

type ArrQueueRecord = {
  id: number;
  title: string;
  protocol: string;
  indexer?: string;
  customFormatScore: number;
  downloadClient: string;
  trackedDownloadState?: string;
  statusMessages?: { title: string; messages: string[] }[];
};

type Props = {
  instanceId: string;
  // stable labels for copy/links/api
  serviceName: "Sonarr" | "Radarr";
  queuePath: "/api/sonarr/queue" | "/api/radarr/queue";
  // service.stats[serviceKey].queue
  getQueue: (
    stats: ServiceStats
  ) => { totalRecords: number; records: ArrQueueRecord[] } | undefined;
  // allow Radarr importPending as well
  canManageRecord: (record: ArrQueueRecord) => boolean;
  getManageDisabledReason: (record: ArrQueueRecord) => string;
  renderMessage: (props: { status: ServiceStatus; message?: string }) => React.ReactNode;
};

export const ArrQueueStatsBase: React.FC<Props> = ({
  instanceId,
  serviceName,
  queuePath,
  getQueue,
  canManageRecord,
  getManageDisabledReason,
  renderMessage,
}) => {
  const { getService, refreshService } = useServiceData();
  const service = getService(instanceId);
  const isLoading = service?.status === "loading";

  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [selectedItem, setSelectedItem] = useState<ArrQueueRecord | null>(null);
  const [deleteOptions, setDeleteOptions] = useState<ArrQueueDeleteOptions>({
    removeFromClient: "change",
    blocklist: "none",
  });

  const queue = service?.stats ? getQueue(service.stats) : undefined;

  const openUrl = service?.accessUrl || service?.url;

  const handleDelete = async () => {
    if (!selectedItem) return;

    try {
      const queryParams = buildArrQueueDeleteQueryParams(instanceId, deleteOptions);

      await api.delete(`${queuePath}/${selectedItem.id}?${queryParams.toString()}`);

      await refreshService(instanceId, "stats");

      setShowDeleteModal(false);
      setSelectedItem(null);
      toast.custom((t) => <Toast type="success" body="Successfully removed from queue" t={t} />);
    } catch (error) {
      console.error("Failed to delete queue item:", error);
      toast.custom((t) => <Toast type="error" body="Failed to remove from queue" t={t} />);
    }
  };

  if (isLoading) {
    return <StatsSkeleton rows={3} />;
  }

  if (!service) {
    return null;
  }

  return (
    <div className="space-y-4">
      {renderMessage({ status: service.status, message: service.message })}

      {queue && queue.totalRecords > 0 && (
        <div>
          <div className="text-xs mb-2 font-semibold text-gray-700 dark:text-gray-300 cursor-default">
            Queue ({queue.totalRecords}):
          </div>

          <div className="space-y-2">
            {queue.records.slice(0, 3).map((record) => (
              <div
                key={record.id}
                className="flex flex-col p-3 text-sm rounded-lg bg-gray-850/95 dark:bg-gray-850/95"
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
                    {openUrl && (
                      <a
                        href={`${openUrl}/activity/queue`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="p-1.5 rounded-md hover:bg-gray-700 dark:hover:bg-gray-700 transition-colors"
                        title={`View in ${serviceName}`}
                      >
                        <ArrowTopRightOnSquareIcon className="h-4 w-4 text-gray-400" />
                      </a>
                    )}

                    <button
                      onClick={() => {
                        setSelectedItem(record);
                        setShowDeleteModal(true);
                      }}
                      disabled={!canManageRecord(record)}
                      className={`p-1.5 rounded-md transition-colors ${
                        canManageRecord(record)
                          ? "hover:bg-gray-700 dark:hover:bg-gray-700"
                          : "opacity-50 cursor-not-allowed"
                      }`}
                      title={
                        canManageRecord(record)
                          ? "Manage queue"
                          : getManageDisabledReason(record)
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

                {record.statusMessages && record.statusMessages.length > 0 && (
                  <div className="mt-2 w-full space-y-1 pointer-events-none">
                    {record.statusMessages.map((msg, idx) => (
                      <div key={idx} className="space-y-1 w-full">
                        {msg.messages?.map((message, msgIdx) => (
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
            {openUrl ? (
              <a
                href={`${openUrl}/activity/queue`}
                target="_blank"
                rel="noopener noreferrer"
                className="font-bold px-2 py-1 rounded-md bg-gray-750 text-gray-700 dark:text-gray-300 break-all inline-block hover:bg-gray-700 transition-colors"
              >
                {selectedItem?.title}
              </a>
            ) : (
              <span className="font-bold px-2 py-1 rounded-md bg-gray-750 text-gray-700 dark:text-gray-300 break-all inline-block">
                {selectedItem?.title}
              </span>
            )}
          </p>

          {selectedItem?.statusMessages && selectedItem.statusMessages.length > 0 && (
            <div className="space-y-1">
              {selectedItem.statusMessages.map((msg, idx) => (
                <div key={idx} className="text-xs break-all text-amber-300">
                  {msg.messages?.map((message, msgIdx) => (
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
                      removeFromClient: value as ArrQueueDeleteOptions["removeFromClient"],
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
                                active ? "bg-gray-600 text-gray-200" : "text-gray-300"
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
                                  active ? "bg-gray-600 text-gray-200" : "text-gray-300"
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
                                active ? "bg-gray-600 text-gray-200" : "text-gray-300"
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
                      blocklist: value as ArrQueueDeleteOptions["blocklist"],
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
                        <Listbox.Options className="absolute z-10 mt-1 max-h-60 w-full overflow-auto rounded-md bg-gray-700 py-1 shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none sm:text-xs">
                          <Listbox.Option
                            value="none"
                            className={({ active }) =>
                              `relative cursor-pointer select-none py-2 pl-3 pr-9 transition-colors ${
                                active ? "bg-gray-600 text-gray-200" : "text-gray-300"
                              }`
                            }
                          >
                            Do not blocklist
                          </Listbox.Option>

                          <Listbox.Option
                            value="block"
                            className={({ active }) =>
                              `relative cursor-pointer select-none py-2 pl-3 pr-9 transition-colors ${
                                active ? "bg-gray-600 text-gray-200" : "text-gray-300"
                              }`
                            }
                          >
                            Add to blocklist
                          </Listbox.Option>

                          <Listbox.Option
                            value="blacklist"
                            className={({ active }) =>
                              `relative cursor-pointer select-none py-2 pl-3 pr-9 transition-colors ${
                                active ? "bg-gray-600 text-gray-200" : "text-gray-300"
                              }`
                            }
                          >
                            Add to blacklist
                          </Listbox.Option>
                        </Listbox.Options>
                      </Transition>
                    </div>
                  )}
                </Listbox>

                <p className="text-xs text-gray-500 mt-1">
                  {getBlocklistText(deleteOptions.blocklist)}
                </p>
              </div>
            </div>
          </div>

          <div className="flex justify-end space-x-3 pt-4">
            <button
              onClick={() => {
                setShowDeleteModal(false);
                setSelectedItem(null);
              }}
              className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-200 dark:bg-gray-700 rounded-md hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleDelete}
              className="px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-md hover:bg-red-700 transition-colors"
            >
              Remove
            </button>
          </div>
        </div>
      </AnimatedModal>
    </div>
  );
};
