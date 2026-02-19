/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { ChevronUpIcon } from "@heroicons/react/24/outline";

interface CollapsibleSectionProps {
  title: React.ReactNode;
  meta?: React.ReactNode;
  isExpanded: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}

export const CollapsibleSection: React.FC<CollapsibleSectionProps> = ({
  title,
  meta,
  isExpanded,
  onToggle,
  children,
}) => {
  const headerSpacingClass = isExpanded ? "mb-2" : "mb-0";

  return (
    <div>
      <div
        onClick={onToggle}
        className={`relative flex w-full cursor-pointer select-none items-center justify-between text-xs font-semibold text-gray-700 group dark:text-gray-300 ${headerSpacingClass}`}
      >
        {meta ? (
          <div className="flex min-w-0 flex-1 items-center justify-between pr-6">
            <span className="truncate">{title}</span>
            <span className="ml-2 flex-shrink-0 text-[11px] font-medium text-gray-500 dark:text-gray-400">
              {meta}
            </span>
          </div>
        ) : (
          <span>{title}</span>
        )}
        <div className="absolute right-0 top-1/2 -translate-y-1/2 pr-0.5 text-gray-500 transition-transform duration-200">
          <ChevronUpIcon
            className={`h-3.5 w-3.5 transform transition-transform duration-200 ${
              isExpanded ? "rotate-180" : ""
            } group-hover:text-gray-400`}
          />
        </div>
      </div>
      <div
        className={`overflow-hidden transition-[max-height,opacity] duration-200 ease-in-out ${
          isExpanded ? "max-h-[1000px] opacity-100" : "max-h-0 opacity-0"
        }`}
      >
        {children}
      </div>
    </div>
  );
};
