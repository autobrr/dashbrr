/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";

interface ServiceAction {
  label: string;
  onClick: () => void;
  variant?: "primary" | "secondary" | "danger";
  icon?: React.ReactNode;
  disabled?: boolean;
}

interface ServiceActionsProps {
  actions: ServiceAction[];
  className?: string;
}

export const ServiceActions: React.FC<ServiceActionsProps> = ({
  actions,
  className = "",
}) => {
  const getVariantClasses = (variant: ServiceAction["variant"] = "primary") => {
    const variants = {
      primary: "bg-blue-500 hover:bg-blue-600 text-white",
      secondary:
        "bg-gray-200 hover:bg-gray-300 text-gray-800 dark:bg-gray-700 dark:hover:bg-gray-600 dark:text-white",
      danger: "bg-red-500 hover:bg-red-600 text-white",
    } as const;

    return variants[variant];
  };

  return (
    <div className={`flex flex-wrap gap-2 ${className}`}>
      {actions.map((action, index) => (
        <button
          key={index}
          onClick={action.onClick}
          disabled={action.disabled}
          className={`
            px-3 py-1.5 rounded-md text-sm font-medium
            transition-colors duration-200
            flex items-center gap-1.5
            disabled:opacity-50 disabled:cursor-not-allowed
            ${getVariantClasses(action.variant)}
          `}
        >
          {action.icon}
          {action.label}
        </button>
      ))}
    </div>
  );
};
