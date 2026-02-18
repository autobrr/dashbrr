/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { ChevronUpIcon } from "@heroicons/react/24/outline";

interface CollapsibleSectionProps {
  title: string;
  isExpanded: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}

export const CollapsibleSection: React.FC<CollapsibleSectionProps> = ({
  title,
  isExpanded,
  onToggle,
  children,
}) => {
  return (
    <div>
      <div
        onClick={onToggle}
        className="relative mb-2 flex w-full cursor-pointer select-none items-center justify-between text-xs font-semibold text-gray-700 group dark:text-gray-300"
      >
        <span>{title}</span>
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
