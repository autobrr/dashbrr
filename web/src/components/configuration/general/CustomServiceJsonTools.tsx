/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { toast } from "react-hot-toast";
import type { CustomServiceConfig } from "../../../types/service";
import { Button } from "../../ui/Button";
import { testGeneralService, type GeneralTestResult } from "../../../api/general";
import { parseCustomServiceConfigJSON, redactForExport } from "./customServiceConfig";
import { TextAreaField } from "./fields";

interface CustomServiceJsonToolsProps {
  config: CustomServiceConfig;
  onImport: (config: CustomServiceConfig) => void;
  url: string;
  apiKey: string;
}

export const CustomServiceJsonTools: React.FC<CustomServiceJsonToolsProps> = ({
  config,
  onImport,
  url,
  apiKey,
}) => {
  const [importText, setImportText] = React.useState("");
  const [importErrors, setImportErrors] = React.useState<string[]>([]);
  const [isTesting, setIsTesting] = React.useState(false);
  const [testResult, setTestResult] = React.useState<GeneralTestResult | null>(null);
  const [testError, setTestError] = React.useState<string | null>(null);

  const handleImport = () => {
    setImportErrors([]);
    if (!importText.trim()) {
      setImportErrors(["Paste a JSON definition to import"]);
      return;
    }
    const result = parseCustomServiceConfigJSON(importText);
    if (!result.ok || !result.config) {
      setImportErrors(result.errors);
      return;
    }
    onImport(result.config);
    setImportText("");
    toast.success("Imported custom service definition");
  };

  const handleExport = async () => {
    const json = JSON.stringify(redactForExport(config), null, 2);
    try {
      await navigator.clipboard.writeText(json);
      toast.success("Copied definition JSON to clipboard (credentials excluded)");
    } catch {
      // Clipboard API may be unavailable (permissions, non-secure context);
      // fall back to showing it in the import box so it can be selected.
      setImportText(json);
      toast("Clipboard unavailable - definition JSON (credentials excluded) shown below");
    }
  };

  const handleTest = async () => {
    if (!url) {
      toast.error("Enter a URL before testing");
      return;
    }
    setIsTesting(true);
    setTestError(null);
    setTestResult(null);
    try {
      const result = await testGeneralService({ url, apiKey, config });
      setTestResult(result);
      if (result.error) {
        setTestError(result.error);
      }
    } catch (err) {
      setTestError(err instanceof Error ? err.message : "Test failed");
    } finally {
      setIsTesting(false);
    }
  };

  return (
    <div className="space-y-3">
      <TextAreaField
        id="general-import-json"
        label="Import JSON"
        value={importText}
        onChange={setImportText}
        placeholder="Paste a custom service definition JSON"
        rows={6}
      />

      {importErrors.length > 0 && (
        <ul className="list-disc pl-5 text-sm text-red-600 dark:text-red-400">
          {importErrors.map((error) => (
            <li key={error}>{error}</li>
          ))}
        </ul>
      )}

      <div className="flex flex-wrap gap-3">
        <Button type="button" variant="secondary" size="sm" onClick={handleImport}>
          Validate &amp; import
        </Button>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={handleExport}
          title="Credentials (auth password/token, login body) are excluded from the exported JSON"
        >
          Export JSON
        </Button>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={handleTest}
          isLoading={isTesting}
        >
          Test
        </Button>
      </div>

      {(testResult || testError) && (
        <div className="rounded-md border border-gray-200 dark:border-gray-700 p-3 text-sm">
          {testError && (
            <p className="text-red-600 dark:text-red-400">{testError}</p>
          )}
          {testResult && (
            <div className="space-y-1 text-gray-700 dark:text-gray-300">
              <p>
                Status: <span className="font-medium">{testResult.status}</span>
              </p>
              {testResult.version && (
                <p>
                  Version: <span className="font-medium">{testResult.version}</span>
                </p>
              )}
              {testResult.stats && Object.keys(testResult.stats).length > 0 && (
                <ul className="list-disc pl-5">
                  {Object.entries(testResult.stats).map(([label, stat]) => (
                    <li key={label}>
                      {label}: {stat.display}
                      {stat.unit ? ` ${stat.unit}` : ""}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
