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

  // okValues/warnValues are kept as raw text in local state and only
  // parsed into an array on blur. Deriving the displayed value straight
  // from the parsed array (the previous approach) re-renders the input
  // from toStringList's trimmed/filtered output on every keystroke, so
  // typing a trailing "," is immediately dropped and the user can never
  // start a second value. These only resync from the `health` prop when
  // its array reference actually changes (an external reset - e.g.
  // switching services or importing JSON - not our own onBlur commit,
  // which doesn't change the prop until the parent re-renders with it).
  const [okText, setOkText] = React.useState(
    (health.okValues || []).join(", ")
  );
  const [warnText, setWarnText] = React.useState(
    (health.warnValues || []).join(", ")
  );

  React.useEffect(() => {
    setOkText((health.okValues || []).join(", "));
  }, [health.okValues]);

  React.useEffect(() => {
    setWarnText((health.warnValues || []).join(", "));
  }, [health.warnValues]);

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
        value={okText}
        onChange={(e) => setOkText(e.target.value)}
        onBlur={(e) => patch({ okValues: toStringList(e.target.value) })}
        // Commit-on-blur alone leaves a gap: pressing Enter to submit the
        // form while focused here fires the submit handler (which reads
        // `health` from parent state) before blur ever runs, so the
        // just-typed text would be lost. Commit here too - and don't
        // preventDefault, so Enter still submits the form as before.
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            patch({ okValues: toStringList(e.currentTarget.value) });
          }
        }}
        placeholder="ok, healthy, running"
      />
      <FormInput
        id="general-health-warn-values"
        label="Warning values (comma separated)"
        type="text"
        value={warnText}
        onChange={(e) => setWarnText(e.target.value)}
        onBlur={(e) => patch({ warnValues: toStringList(e.target.value) })}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            patch({ warnValues: toStringList(e.currentTarget.value) });
          }
        }}
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
