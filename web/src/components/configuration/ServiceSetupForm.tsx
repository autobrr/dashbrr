/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useState } from "react";
import { toast } from "react-hot-toast";

import {
  ServiceSetup,
  serviceTestNeeded,
  setupInput,
  SetupValues
} from "../../config/serviceSetup";
import { usePlexPinAuth } from "../../hooks/usePlexPinAuth";
import { ServiceConfigInput } from "../../types/service";
import { testService } from "../../utils/serviceTest";
import { Button } from "../ui/Button";
import { FormInput } from "../ui/FormInput";

interface ServiceSetupFormProps {
  setup: ServiceSetup;
  submitLabel: string;
  onSubmit: (input: ServiceConfigInput) => Promise<void>;
  onCancel: () => void;
}

export const ServiceSetupForm = ({
  setup,
  submitLabel,
  onSubmit,
  onCancel,
}: ServiceSetupFormProps) => {
  const [values, setValues] = useState<SetupValues>(setup.values);
  const [phase, setPhase] = useState<"idle" | "testing" | "saving">("idle");
  const isSubmitting = phase !== "idle";
  const [error, setError] = useState<string | null>(null);
  const { isAuthenticating, authenticate } = usePlexPinAuth();
  const { form, fields, required } = setup;

  const set = (field: keyof SetupValues) => (value: string) =>
    setValues((prev) => ({ ...prev, [field]: value }));

  const fail = (message: string) => {
    toast.error(message);
    setError(message);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const result = setupInput(setup, values);
    if ("error" in result) {
      fail(result.error);
      return;
    }

    try {
      if (serviceTestNeeded(setup, result.input)) {
        setPhase("testing");
        await testService(setup.instanceId, result.input);
      }
      setPhase("saving");
      await onSubmit(result.input);
    } catch (err) {
      fail(err instanceof Error ? err.message : "Failed to save configuration");
    } finally {
      setPhase("idle");
    }
  };

  if (setup.edits === "none") {
    return (
      <div className="space-y-4">
        <p className="text-sm text-gray-600 dark:text-gray-300">
          Kubernetes discovery manages this service. Change its annotations to change it.
        </p>
        <div className="flex justify-end">
          <Button type="button" variant="secondary" onClick={onCancel}>
            Close
          </Button>
        </div>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {fields.includes("displayName") && (
        <FormInput
          id="displayName"
          label="Display Name"
          type="text"
          value={values.displayName}
          onChange={(e) => set("displayName")(e.target.value)}
          placeholder="Enter display name"
          required={required.includes("displayName")}
        />
      )}

      {fields.includes("url") && (
        <FormInput
          id="url"
          label="URL"
          type="text"
          value={values.url}
          onChange={(e) => set("url")(e.target.value)}
          placeholder={form.urlPlaceholder}
          helpText={{
            prefix: "Used for ",
            text: "API communication and health checks",
            link: null,
          }}
          required={required.includes("url")}
          data-1p-ignore
        />
      )}

      {fields.includes("accessUrl") && (
        <FormInput
          id="accessUrl"
          label="Access URL (Optional)"
          type="text"
          value={values.accessUrl}
          onChange={(e) => set("accessUrl")(e.target.value)}
          placeholder="Leave empty to use main URL"
          helpText={{
            prefix: "Override ",
            text: "URL used when opening service in browser. Useful for internal/external URL differences.",
            link: null,
          }}
          data-1p-ignore
        />
      )}

      {fields.includes("apiKey") && (
        <FormInput
          id="apiKey"
          label={form.apiKeyLabel}
          type="password"
          value={values.apiKey}
          onChange={(e) => set("apiKey")(e.target.value)}
          placeholder={`Enter ${form.apiKeyLabel}`}
          helpText={form.apiKeyHelp({ url: values.url, accessUrl: values.accessUrl })}
          required={required.includes("apiKey")}
          data-1p-ignore
        />
      )}

      {fields.includes("plexSignIn") && (
        <div className="space-y-2">
          <Button
            type="button"
            variant="secondary"
            onClick={async () => {
              try {
                await authenticate(set("apiKey"));
              } catch (err) {
                fail(
                  err instanceof Error
                    ? err.message
                    : "Failed to start Plex authentication"
                );
              }
            }}
            disabled={isSubmitting || isAuthenticating}
          >
            {isAuthenticating ? "Waiting for Plex login..." : "Authenticate with Plex"}
          </Button>
          {values.apiKey.trim() !== "" && (
            <div className="text-xs text-emerald-400">Authenticated with Plex</div>
          )}
        </div>
      )}

      {error && (
        <div className="text-red-600 dark:text-red-400 text-sm">{error}</div>
      )}

      <div className="flex justify-end space-x-3">
        <Button
          type="button"
          variant="secondary"
          onClick={onCancel}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        <Button variant="primary" type="submit" isLoading={isSubmitting}>
          {phase === "testing"
            ? "Testing..."
            : phase === "saving"
              ? "Saving..."
              : submitLabel}
        </Button>
      </div>
    </form>
  );
};
