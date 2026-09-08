/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { TrashIcon } from "@heroicons/react/20/solid";
import type { CustomActionConfig, CustomActionMethod } from "../../../types/service";
import { Button } from "../../ui/Button";
import { CheckboxField, SelectField, TextField } from "./fields";

interface CustomServiceActionsEditorProps {
  actions: CustomActionConfig[];
  onChange: (actions: CustomActionConfig[]) => void;
  maxEntries?: number;
}

const METHOD_OPTIONS: Array<{ value: CustomActionMethod | ""; label: string }> = [
  { value: "", label: "POST (default)" },
  { value: "GET", label: "GET" },
  { value: "POST", label: "POST" },
  { value: "PUT", label: "PUT" },
  { value: "DELETE", label: "DELETE" },
];

export const CustomServiceActionsEditor: React.FC<CustomServiceActionsEditorProps> = ({
  actions,
  onChange,
  maxEntries = 8,
}) => {
  const updateRow = (index: number, fields: Partial<CustomActionConfig>) => {
    const next = actions.map((row, i) => (i === index ? { ...row, ...fields } : row));
    onChange(next);
  };

  const removeRow = (index: number) => {
    onChange(actions.filter((_, i) => i !== index));
  };

  const addRow = () => {
    if (actions.length >= maxEntries) return;
    onChange([...actions, { id: "", label: "", path: "" }]);
  };

  return (
    <div className="space-y-3">
      {actions.map((action, index) => (
        <div
          key={index}
          className="space-y-2 rounded-md border border-gray-200 dark:border-gray-700 p-3"
        >
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_1fr_auto_auto]">
            <TextField
              id={`general-action-id-${index}`}
              label="ID"
              value={action.id}
              onChange={(v) => updateRow(index, { id: v })}
              placeholder="restart-service"
            />
            <TextField
              id={`general-action-label-${index}`}
              label="Label"
              value={action.label}
              onChange={(v) => updateRow(index, { label: v })}
              placeholder="Restart service"
            />
            <SelectField
              id={`general-action-method-${index}`}
              label="Method"
              value={action.method || ""}
              onChange={(v) => updateRow(index, { method: v as CustomActionMethod | "" })}
              options={METHOD_OPTIONS}
            />
            <div className="flex items-end">
              <button
                type="button"
                onClick={() => removeRow(index)}
                className="rounded-full p-2 text-red-500 hover:bg-red-50 hover:text-red-700 dark:hover:bg-red-500/20"
                title="Remove action"
                aria-label={`Remove action ${index + 1}`}
              >
                <TrashIcon className="h-4 w-4" />
              </button>
            </div>
          </div>
          <TextField
            id={`general-action-path-${index}`}
            label="Path"
            value={action.path}
            onChange={(v) => updateRow(index, { path: v })}
            placeholder="/api/v1/restart"
          />
          <TextField
            id={`general-action-body-${index}`}
            label="Body (optional)"
            value={action.body || ""}
            onChange={(v) => updateRow(index, { body: v })}
            placeholder='{"force":true}'
          />
          <CheckboxField
            id={`general-action-confirm-${index}`}
            label="Require confirmation before running"
            checked={Boolean(action.confirm)}
            onChange={(v) => updateRow(index, { confirm: v })}
          />
        </div>
      ))}

      <Button
        type="button"
        variant="secondary"
        size="sm"
        onClick={addRow}
        disabled={actions.length >= maxEntries}
      >
        Add action ({actions.length}/{maxEntries})
      </Button>
    </div>
  );
};
