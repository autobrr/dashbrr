/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import {
  ServiceConfig,
  ServiceConfigInput,
  ServiceEdits,
  ServiceType
} from "../types/service";
import { ServiceForm, serviceFormFor, serviceForms } from "./serviceForms";

export type SetupField =
  | "displayName"
  | "url"
  | "accessUrl"
  | "apiKey"
  | "plexSignIn";

export interface SetupValues {
  displayName: string;
  url: string;
  accessUrl: string;
  apiKey: string;
}

export interface ServiceSetup {
  mode: "add" | "edit";
  instanceId: string;
  form: ServiceForm;
  edits: ServiceEdits;
  fields: SetupField[];
  required: SetupField[];
  values: SetupValues;
}

export type SetupResult = { input: ServiceConfigInput } | { error: string };

type Configurations = Record<string, ServiceConfig>;

const credentialField = (form: ServiceForm): SetupField =>
  form.credential === "plexSignIn" ? "plexSignIn" : "apiKey";

const fieldsFor = (form: ServiceForm, edits: ServiceEdits): SetupField[] => {
  switch (edits) {
    case "none":
      return [];
    case "apiKey":
      return [credentialField(form)];
    case "all":
      return [
        "displayName",
        ...(form.hideUrl ? [] : (["url"] as const)),
        "accessUrl",
        credentialField(form),
      ];
  }
};

const requiredFields = (
  form: ServiceForm,
  fields: SetupField[],
  mode: ServiceSetup["mode"]
): SetupField[] =>
  fields.filter((field) => {
    switch (field) {
      case "displayName":
      case "url":
        return true;
      case "accessUrl":
        return false;
      case "apiKey":
        return mode === "add" && form.credential !== "optional";
      case "plexSignIn":
        return mode === "add";
    }
  });

export const freeInstanceId = (
  type: ServiceType,
  configurations: Configurations
): { instanceId: string; number: number } => {
  for (let number = 1; ; number++) {
    const instanceId = `${type}-${number}`;
    if (!Object.prototype.hasOwnProperty.call(configurations, instanceId)) {
      return { instanceId, number };
    }
  }
};

export const addServiceSetup = (
  type: ServiceType,
  name: string,
  configurations: Configurations
): ServiceSetup => {
  const form = serviceForms[type];
  const { instanceId, number } = freeInstanceId(type, configurations);
  const fields = fieldsFor(form, "all");

  return {
    mode: "add",
    instanceId,
    form,
    edits: "all",
    fields,
    required: requiredFields(form, fields, "add"),
    values: {
      displayName:
        type === "general" ? "" : number > 1 ? `${name} ${number}` : name,
      url: form.defaultUrl ?? "",
      accessUrl: "",
      apiKey: "",
    },
  };
};

export const editServiceSetup = (
  instanceId: string,
  configurations: Configurations
): ServiceSetup => {
  const config = configurations[instanceId];
  const form = serviceFormFor(instanceId.split("-")[0]);
  const edits = config?.edits ?? "all";
  const fields = fieldsFor(form, edits);

  return {
    mode: "edit",
    instanceId,
    form,
    edits,
    fields,
    required: requiredFields(form, fields, "edit"),
    values: {
      displayName: config?.displayName ?? "",
      url: (form.hideUrl ? form.defaultUrl : config?.url) ?? "",
      accessUrl: config?.accessUrl ?? "",
      apiKey: "",
    },
  };
};

const fieldValue = (values: SetupValues, field: SetupField): string =>
  field === "plexSignIn" ? values.apiKey : values[field];

const missingMessage = (setup: ServiceSetup, field: SetupField): string => {
  switch (field) {
    case "plexSignIn":
      return "Authenticate with Plex first";
    case "apiKey":
      return `${setup.form.apiKeyLabel} is required`;
    case "displayName":
      return "Display Name is required";
    case "url":
      return "URL is required";
    case "accessUrl":
      return "Access URL is required";
  }
};

export const setupInput = (
  setup: ServiceSetup,
  values: SetupValues
): SetupResult => {
  const normalized: SetupValues = {
    displayName: values.displayName.trim(),
    url: values.url.trim(),
    accessUrl: values.accessUrl.trim(),
    apiKey: values.apiKey,
  };
  const missing = setup.required.find(
    (field) => fieldValue(normalized, field).trim() === ""
  );
  if (missing) {
    return { error: missingMessage(setup, missing) };
  }

  return {
    input: {
      ...normalized,
      accessUrl: normalized.accessUrl === "" ? undefined : normalized.accessUrl,
    },
  };
};

export const serviceTestNeeded = (
  setup: ServiceSetup,
  input: ServiceConfigInput
): boolean => {
  if (setup.form.testBeforeSave === false) return false;
  if (setup.mode === "add") return true;
  return input.url !== setup.values.url || input.apiKey !== "";
};
