/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import type { CustomHealthConfig } from "../../../types/service";
import { FormInput } from "../../ui/FormInput";
import { SelectField, TextAreaField } from "./fields";

interface CustomServiceHealthFieldsProps {
  health: CustomHealthConfig;
  onChange: (health: CustomHealthConfig) => void;
}

const toStringList = (value: string): string[] =>
  value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);

export const CustomServiceHealthFields: React.FC<CustomServiceHealthFieldsProps> = ({
  health,
  onChange,
}) => {
  const patch = (fields: Partial<CustomHealthConfig>) =>
    onChange({ ...health, ...fields });

  return (
    <div className="space-y-1">
      <SelectField
        id="general-health-method"
        label="Method"
        value={health.method || "GET"}
        onChange={(v) => patch({ method: v as CustomHealthConfig["method"] })}
        options={[
          { value: "GET", label: "GET" },
          { value: "POST", label: "POST" },
        ]}
      />
      <FormInput
        id="general-health-path"
        label="Path"
        type="text"
        value={health.path || ""}
        onChange={(e) => patch({ path: e.target.value })}
        placeholder="/api/health"
        helpText={{
          prefix: "",
          text: "Required only if you configure a health check here. Leave the whole health block unset to use the service's default health handling.",
          link: null,
        }}
      />
      {health.method === "POST" && (
        <TextAreaField
          id="general-health-body"
          label="Body"
          value={health.body || ""}
          onChange={(v) => patch({ body: v })}
        />
      )}
      <FormInput
        id="general-health-status-path"
        label="Status JSON path"
        type="text"
        value={health.statusPath || ""}
        onChange={(e) => patch({ statusPath: e.target.value })}
        placeholder="Leave empty to treat HTTP 2xx as online"
        helpText={{
          prefix: "",
          text: "gjson path, e.g. status or data.state",
          link: null,
        }}
      />
      <FormInput
        id="general-health-ok-values"
        label="OK values (comma separated)"
        type="text"
        value={(health.okValues || []).join(", ")}
        onChange={(e) => patch({ okValues: toStringList(e.target.value) })}
        placeholder="ok, healthy, running"
      />
      <FormInput
        id="general-health-warn-values"
        label="Warning values (comma separated)"
        type="text"
        value={(health.warnValues || []).join(", ")}
        onChange={(e) => patch({ warnValues: toStringList(e.target.value) })}
        placeholder="degraded, warning"
      />
      <FormInput
        id="general-health-version-path"
        label="Version JSON path"
        type="text"
        value={health.versionPath || ""}
        onChange={(e) => patch({ versionPath: e.target.value })}
        placeholder="version"
      />
    </div>
  );
};
