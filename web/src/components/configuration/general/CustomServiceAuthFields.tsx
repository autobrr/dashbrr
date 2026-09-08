/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import type { CustomAuthConfig, CustomAuthMode } from "../../../types/service";
import { FormInput } from "../../ui/FormInput";
import { SelectField } from "./fields";

interface CustomServiceAuthFieldsProps {
  auth: CustomAuthConfig;
  onChange: (auth: CustomAuthConfig) => void;
  // True when a login step is configured. The login step's body can
  // reference {{username}}/{{password}} placeholders that are filled from
  // auth.username/auth.password, so those fields must stay visible whenever
  // a login step is enabled - not only when auth.mode is "basic".
  loginEnabled?: boolean;
}

const AUTH_MODE_OPTIONS: Array<{ value: CustomAuthMode; label: string }> = [
  { value: "none", label: "None" },
  { value: "header", label: "Custom header" },
  { value: "query", label: "Query parameter" },
  { value: "basic", label: "Basic auth (username/password)" },
  { value: "bearer", label: "Bearer token" },
];

export const CustomServiceAuthFields: React.FC<CustomServiceAuthFieldsProps> = ({
  auth,
  onChange,
  loginEnabled = false,
}) => {
  const patch = (fields: Partial<CustomAuthConfig>) =>
    onChange({ ...auth, ...fields });
  const showCredentialFields = auth.mode === "basic" || loginEnabled;

  return (
    <div className="space-y-1">
      <SelectField
        id="general-auth-mode"
        label="Authentication mode"
        value={auth.mode || "none"}
        onChange={(value) => patch({ mode: value as CustomAuthMode })}
        options={AUTH_MODE_OPTIONS}
      />

      {auth.mode === "header" && (
        <>
          <FormInput
            id="general-auth-header-name"
            label="Header name"
            type="text"
            value={auth.headerName || ""}
            onChange={(e) => patch({ headerName: e.target.value })}
            placeholder="X-Api-Key"
          />
          <FormInput
            id="general-auth-token"
            label="Header value"
            type="password"
            value={auth.token || ""}
            onChange={(e) => patch({ token: e.target.value })}
            helpText={{
              prefix: "",
              text: `Value sent in the ${auth.headerName || "configured"} header. Leave blank to keep the stored value when editing an existing service.`,
              link: null,
            }}
          />
        </>
      )}

      {auth.mode === "query" && (
        <>
          <FormInput
            id="general-auth-query-param"
            label="Query parameter name"
            type="text"
            value={auth.queryParam || ""}
            onChange={(e) => patch({ queryParam: e.target.value })}
            placeholder="apikey"
          />
          <FormInput
            id="general-auth-token-query"
            label="Query parameter value"
            type="password"
            value={auth.token || ""}
            onChange={(e) => patch({ token: e.target.value })}
            helpText={{
              prefix: "",
              text: `Value sent as the ${auth.queryParam || "configured"} query parameter. Leave blank to keep the stored value when editing an existing service.`,
              link: null,
            }}
          />
        </>
      )}

      {showCredentialFields && (
        <>
          <FormInput
            id="general-auth-username"
            label="Username"
            type="text"
            value={auth.username || ""}
            onChange={(e) => patch({ username: e.target.value })}
            helpText={
              auth.mode !== "basic"
                ? {
                  prefix: "",
                  text: "Used to fill {{username}} in the login step body",
                  link: null,
                }
                : undefined
            }
          />
          <FormInput
            id="general-auth-password"
            label="Password"
            type="password"
            value={auth.password || ""}
            onChange={(e) => patch({ password: e.target.value })}
            helpText={
              auth.mode !== "basic"
                ? {
                  prefix: "",
                  text: "Used to fill {{password}} in the login step body",
                  link: null,
                }
                : undefined
            }
          />
        </>
      )}

      {auth.mode === "bearer" && (
        <FormInput
          id="general-auth-bearer-token"
          label="Bearer token"
          type="password"
          value={auth.token || ""}
          onChange={(e) => patch({ token: e.target.value })}
          helpText={{
            prefix: "",
            text: "Sent as Authorization: Bearer <token>. Leave blank to keep the stored value when editing an existing service.",
            link: null,
          }}
        />
      )}
    </div>
  );
};
