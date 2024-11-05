import React from "react";
import {
  triggerWebhookArrs,
  triggerWebhookLists,
  triggerWebhookAll,
} from "../../../config/api";
import { toast } from "react-hot-toast";

interface OmegabrrControlsProps {
  url: string;
  apiKey: string;
}

export const OmegabrrControls: React.FC<OmegabrrControlsProps> = ({
  url,
  apiKey,
}) => {
  const handleTriggerArrs = async () => {
    if (!apiKey || !url) {
      toast.error("Service URL and API key must be configured first.");
      return;
    }

    try {
      await triggerWebhookArrs(url, apiKey);
      toast.success("ARRs webhook triggered successfully");
    } catch (err) {
      console.error("Failed to trigger ARRs webhook:", err);
      toast.error(
        "Failed to trigger ARRs webhook. Check the console for details."
      );
    }
  };

  const handleTriggerLists = async () => {
    if (!apiKey || !url) {
      toast.error("Service URL and API key must be configured first.");
      return;
    }

    try {
      await triggerWebhookLists(url, apiKey);
      toast.success("Lists webhook triggered successfully");
    } catch (err) {
      console.error("Failed to trigger Lists webhook:", err);
      toast.error(
        "Failed to trigger Lists webhook. Check the console for details."
      );
    }
  };

  const handleTriggerAll = async () => {
    if (!apiKey || !url) {
      toast.error("Service URL and API key must be configured first.");
      return;
    }

    try {
      await triggerWebhookAll(url, apiKey);
      toast.success("All webhooks triggered successfully");
    } catch (err) {
      console.error("Failed to trigger all webhooks:", err);
      toast.error(
        "Failed to trigger all webhooks. Check the console for details."
      );
    }
  };

  return (
    <div className="mt-2 space-y-2">
      <h4 className="text-xs font-semibold text-gray-700 dark:text-gray-300">
        Manual Triggers:
      </h4>
      <div className="flex flex-wrap gap-2">
        <button
          onClick={handleTriggerArrs}
          className="px-1.5 py-1 text-xs bg-blue-600 dark:bg-blue-500 text-white rounded hover:bg-blue-700 dark:hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          disabled={!apiKey || !url}
          title={!apiKey ? "Configure API key first" : "Trigger ARRs update"}
        >
          ARRs Update
        </button>
        <button
          onClick={handleTriggerLists}
          className="px-2 py-1 text-xs bg-green-600 dark:bg-green-500 text-white rounded hover:bg-green-700 dark:hover:bg-green-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          disabled={!apiKey || !url}
          title={!apiKey ? "Configure API key first" : "Trigger Lists update"}
        >
          Lists Update
        </button>
        <button
          onClick={handleTriggerAll}
          className="px-2 py-1 text-xs bg-yellow-600 dark:bg-yellow-500 text-white rounded hover:bg-yellow-700 dark:hover:bg-yellow-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          disabled={!apiKey || !url}
          title={!apiKey ? "Configure API key first" : "Trigger all updates"}
        >
          All Updates
        </button>
      </div>
    </div>
  );
};
