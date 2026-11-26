/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React, { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "react-hot-toast";
import { ArrowTopRightOnSquareIcon } from "@heroicons/react/20/solid";
import Toast from "../Toast";
import AnimatedModal from "../ui/AnimatedModal";
import { Button } from "../ui/Button";
import { prepareMembers, type NetworkDescriptor, type NetworkMember } from "../../utils/meshNetwork";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  descriptor: NetworkDescriptor;
  items: NetworkMember[];
}

const CopyIcon = () => (
  <svg
    className="w-4 h-4 ml-1.5 text-gray-400"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
    />
  </svg>
);

const formatLastSeen = (lastSeen: string) => {
  const date = new Date(lastSeen);
  if (Number.isNaN(date.getTime())) {
    return lastSeen || "Unknown";
  }

  return date.toLocaleString();
};

const MeshNetworkListModal: React.FC<Props> = ({
  isOpen,
  onClose,
  descriptor,
  items,
}) => {
  const [searchTerm, setSearchTerm] = useState("");
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Check for CMD+K (Mac) or CTRL+K (Windows/Linux)
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        searchInputRef.current?.focus();
      }
    };

    if (isOpen) {
      window.addEventListener("keydown", handleKeyDown);
      return () => window.removeEventListener("keydown", handleKeyDown);
    }
  }, [isOpen]);

  const visibleItems = useMemo(() => prepareMembers(items, searchTerm), [items, searchTerm]);

  const onlineCount = visibleItems.filter((item) => item.online).length;
  const plural = descriptor.noun.plural;

  // Report success only after the write resolved: a denied clipboard permission
  // rejects, and claiming "copied" then would be a lie.
  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      toast.custom((t) => (
        <Toast type="success" body={`Copied ${text} to clipboard`} t={t} />
      ));
    } catch {
      toast.custom((t) => (
        <Toast type="error" body={`Could not copy ${text} to clipboard`} t={t} />
      ));
    }
  };

  return (
    <AnimatedModal
      isOpen={isOpen}
      onClose={onClose}
      title={
        <div className="flex items-center justify-between">
          <span>{`${descriptor.label} ${plural.charAt(0).toUpperCase()}${plural.slice(1)}`}</span>
          <span className="pl-4 text-sm font-normal text-gray-500 dark:text-gray-400">
            {onlineCount} of {visibleItems.length} online
          </span>
        </div>
      }
      maxWidth="2xl"
      className="bg-gray-950"
    >
      <div className="flex flex-col">
        <div className="relative mt-4 mb-4">
          <input
            ref={searchInputRef}
            type="text"
            placeholder={descriptor.searchPlaceholder}
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full px-3 py-2 pr-20 text-sm border rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-gray-900 border-gray-800 text-gray-300 placeholder-gray-500"
          />
          <div className="absolute inset-y-0 right-2 flex items-center pointer-events-none text-gray-400">
            <kbd className="inline-flex items-center justify-center space-x-1 rounded border border-gray-700 bg-gray-800 px-2 py-1 text-xs font-sans uppercase text-gray-400">
              {navigator.userAgent?.includes("Mac") ? "⌘" : "Ctrl"}+K
            </kbd>
          </div>
        </div>

        <div className="flex-1 -mr-[18px]">
          <div className="max-h-[60vh] overflow-y-auto scrollbar-small pr-[6px]">
            <div className="space-y-2">
              {visibleItems.map((item) => (
                <div
                  key={item.id}
                  className="text-xs rounded-md text-gray-400 bg-gray-900 p-3.5 hover:bg-gray-850 transition-colors"
                >
                  <div className="flex items-start justify-between">
                    <div className="space-y-2 flex-1">
                      <div className="flex flex-col gap-2">
                        <div className="flex items-start justify-between">
                          <div className="flex items-center gap-2 flex-1">
                            <button
                              onClick={() => copyToClipboard(item.name)}
                              className="font-medium text-gray-300 hover:text-blue-400 transition-colors text-left inline-flex items-center cursor-pointer group"
                              title={`Click to copy: ${item.name}`}
                            >
                              <span className="text-base">{item.name}</span>
                              <CopyIcon />
                            </button>
                          </div>
                          <span
                            className={`ml-2 shrink-0 px-2 py-0.5 text-xs font-medium rounded-md ${
                              item.online
                                ? "text-green-400 bg-green-900/30 border border-green-900/50"
                                : "text-red-400 bg-red-900/30 border border-red-900/50"
                            }`}
                          >
                            {item.online ? "Online" : "Offline"}
                          </span>
                        </div>

                        {item.chips && (
                          <div className="flex flex-wrap gap-1 text-gray-300 items-center pointer-events-none">
                            {item.chips.label}:
                            {item.chips.values.map((value) => (
                              <span
                                key={value}
                                className="px-1.5 py-0.5 text-xs font-medium rounded-md bg-blue-900/30 text-blue-400 border border-blue-900/50"
                              >
                                {value}
                              </span>
                            ))}
                          </div>
                        )}

                        {item.expired && (
                          <div className="flex flex-wrap gap-1 text-gray-300 items-center">
                            <span className="px-1.5 py-0.5 text-xs font-medium rounded-md bg-red-900/30 text-red-400 border border-red-900/50">
                              Login expired
                            </span>
                          </div>
                        )}
                      </div>

                      <div className="text-xs space-y-1 cursor-default">
                        {item.details
                          .filter((detail) => detail.value)
                          .map((detail) => (
                            <p key={detail.label} className="text-gray-400">
                              <span className="font-medium text-gray-300">{detail.label}:</span>{" "}
                              {detail.value}
                            </p>
                          ))}
                        <p className="text-gray-400">
                          <span className="font-medium text-gray-300">IP:</span>{" "}
                          <button
                            onClick={() => copyToClipboard(item.ip)}
                            className="hover:text-blue-400 transition-colors inline-flex items-center cursor-pointer group"
                            title="Click to copy IP address"
                          >
                            {item.ip}
                            <CopyIcon />
                          </button>
                        </p>
                        <p className="text-gray-500">
                          <span className="font-medium text-gray-300">Version:</span>{" "}
                          <span className="font-semibold text-gray-400">
                            {item.version ? item.version.split("-")[0] : "Unknown"}
                          </span>
                          {item.updateAvailable && (
                            <>
                              {" - "}
                              <a
                                href={descriptor.updateLink}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-amber-400 inline-flex items-center gap-1 hover:text-amber-300 transition-colors"
                              >
                                Update Available
                                <ArrowTopRightOnSquareIcon className="h-3 w-3" />
                              </a>
                            </>
                          )}
                        </p>
                        <p className="text-gray-450 text-xs">
                          Last seen: {formatLastSeen(item.lastSeen)}
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="mt-6 flex justify-end">
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
      </div>
    </AnimatedModal>
  );
};

export default MeshNetworkListModal;
