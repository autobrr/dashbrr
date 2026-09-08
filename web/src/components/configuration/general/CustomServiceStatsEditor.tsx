/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { TrashIcon } from "@heroicons/react/20/solid";
import type { CustomStatConfig, CustomStatFormat } from "../../../types/service";
import { Button } from "../../ui/Button";
import { SelectField, TextField } from "./fields";

interface CustomServiceStatsEditorProps {
  stats: CustomStatConfig[];
  onChange: (stats: CustomStatConfig[]) => void;
  maxEntries?: number;
}

const FORMAT_OPTIONS: Array<{ value: CustomStatFormat | ""; label: string }> = [
  { value: "", label: "Text (default)" },
  { value: "number", label: "Number" },
  { value: "bytes", label: "Bytes" },
  { value: "duration", label: "Duration" },
  { value: "percent", label: "Percent" },
  { value: "text", label: "Text" },
];

export const CustomServiceStatsEditor: React.FC<CustomServiceStatsEditorProps> = ({
  stats,
  onChange,
  maxEntries = 8,
}) => {
  const updateRow = (index: number, fields: Partial<CustomStatConfig>) => {
    const next = stats.map((row, i) => (i === index ? { ...row, ...fields } : row));
    onChange(next);
  };

  const removeRow = (index: number) => {
    onChange(stats.filter((_, i) => i !== index));
  };

  const addRow = () => {
    if (stats.length >= maxEntries) return;
    onChange([...stats, { label: "", path: "" }]);
  };

  return (
    <div className="space-y-3">
      {stats.map((stat, index) => (
        <div
          key={index}
          className="grid grid-cols-1 gap-2 rounded-md border border-gray-200 dark:border-gray-700 p-3 sm:grid-cols-[1fr_1fr_auto_auto_auto]"
        >
          <TextField
            id={`general-stat-label-${index}`}
            label="Label"
            value={stat.label}
            onChange={(v) => updateRow(index, { label: v })}
            placeholder="Active users"
          />
          <TextField
            id={`general-stat-path-${index}`}
            label="JSON path"
            value={stat.path}
            onChange={(v) => updateRow(index, { path: v })}
            placeholder="data.activeUsers"
          />
          <TextField
            id={`general-stat-unit-${index}`}
            label="Unit"
            value={stat.unit || ""}
            onChange={(v) => updateRow(index, { unit: v })}
            placeholder="users"
          />
          <SelectField
            id={`general-stat-format-${index}`}
            label="Format"
            value={stat.format || ""}
            onChange={(v) => updateRow(index, { format: v as CustomStatFormat | "" })}
            options={FORMAT_OPTIONS}
          />
          <div className="flex items-end">
            <button
              type="button"
              onClick={() => removeRow(index)}
              className="rounded-full p-2 text-red-500 hover:bg-red-50 hover:text-red-700 dark:hover:bg-red-500/20"
              title="Remove stat"
              aria-label={`Remove stat ${index + 1}`}
            >
              <TrashIcon className="h-4 w-4" />
            </button>
          </div>
        </div>
      ))}

      <Button
        type="button"
        variant="secondary"
        size="sm"
        onClick={addRow}
        disabled={stats.length >= maxEntries}
      >
        Add stat ({stats.length}/{maxEntries})
      </Button>
    </div>
  );
};
