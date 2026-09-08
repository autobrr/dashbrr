/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import type { CustomLoginConfig } from "../../../types/service";
import { FormInput } from "../../ui/FormInput";
import { CollapsibleSection } from "../../ui/CollapsibleSection";
import { SelectField, TextAreaField } from "./fields";

interface CustomServiceLoginFieldsProps {
  login: CustomLoginConfig | undefined;
  onChange: (login: CustomLoginConfig | undefined) => void;
}

const emptyLogin: CustomLoginConfig = { method: "POST", injectAs: "header" };

export const CustomServiceLoginFields: React.FC<CustomServiceLoginFieldsProps> = ({
  login,
  onChange,
}) => {
  const [isExpanded, setIsExpanded] = React.useState(Boolean(login));
  const enabled = Boolean(login);
  const value = login || emptyLogin;

  const patch = (fields: Partial<CustomLoginConfig>) =>
    onChange({ ...value, ...fields });

  return (
    <CollapsibleSection
      title="Login step (optional)"
      meta={enabled ? "Configured" : "Not configured"}
      isExpanded={isExpanded}
      onToggle={() => setIsExpanded((prev) => !prev)}
    >
      <div className="space-y-1 pt-2">
        <label className="mb-4 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(e) => onChange(e.target.checked ? emptyLogin : undefined)}
            className="h-4 w-4 rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
          />
          Run a login request before health/stats/actions
        </label>

        {enabled && (
          <>
            <SelectField
              id="general-login-method"
              label="Method"
              value={value.method || "POST"}
              onChange={(v) => patch({ method: v as CustomLoginConfig["method"] })}
              options={[
                { value: "GET", label: "GET" },
                { value: "POST", label: "POST" },
              ]}
            />
            <FormInput
              id="general-login-path"
              label="Path"
              type="text"
              value={value.path || ""}
              onChange={(e) => patch({ path: e.target.value })}
              placeholder="/api/v1/auth/login"
            />
            <FormInput
              id="general-login-content-type"
              label="Content type"
              type="text"
              value={value.contentType || ""}
              onChange={(e) => patch({ contentType: e.target.value })}
              placeholder="application/json"
            />
            <TextAreaField
              id="general-login-body"
              label="Body"
              value={value.body || ""}
              onChange={(v) => patch({ body: v })}
              placeholder='{"username":"...","password":"..."}'
            />
            <FormInput
              id="general-login-capture-cookie"
              label="Capture cookie name"
              type="text"
              value={value.captureCookie || ""}
              onChange={(e) => patch({ captureCookie: e.target.value })}
              placeholder="session"
            />
            <FormInput
              id="general-login-capture-json-path"
              label="Capture JSON path"
              type="text"
              value={value.captureJSONPath || ""}
              onChange={(e) => patch({ captureJSONPath: e.target.value })}
              placeholder="data.token"
            />
            <SelectField
              id="general-login-inject-as"
              label="Inject captured value as"
              value={value.injectAs || "header"}
              onChange={(v) => patch({ injectAs: v as CustomLoginConfig["injectAs"] })}
              options={[
                { value: "header", label: "Header" },
                { value: "query", label: "Query parameter" },
                { value: "cookie", label: "Cookie" },
                { value: "bearer", label: "Bearer token" },
              ]}
            />
            <FormInput
              id="general-login-inject-name"
              label="Inject name"
              type="text"
              value={value.injectName || ""}
              onChange={(e) => patch({ injectName: e.target.value })}
              placeholder="Authorization"
            />
          </>
        )}
      </div>
    </CollapsibleSection>
  );
};
