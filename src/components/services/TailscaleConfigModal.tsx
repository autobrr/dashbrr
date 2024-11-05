import React, { useState, useEffect } from "react";
import { useConfiguration } from "../../contexts/useConfiguration";
import { useAuth } from "../../contexts/AuthContext";
import { useServiceHealth } from "../../hooks/useServiceHealth";
import { toast } from "react-hot-toast";
import { FormInput } from "../ui/FormInput";
import { Button } from "../ui/Button";
import AnimatedModal from "../ui/AnimatedModal";
import { api } from "../../utils/api";

interface TailscaleConfigModalProps {
  isOpen: boolean;
  onClose: () => void;
  configId?: string;
}

export const TailscaleConfigModal: React.FC<TailscaleConfigModalProps> = ({
  isOpen,
  onClose,
  configId,
}) => {
  const { updateConfiguration, deleteConfiguration, configurations } =
    useConfiguration();
  const { refreshServiceHealth } = useServiceHealth();
  const { isAuthenticated } = useAuth();
  const [apiToken, setApiToken] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const instanceId = configId || "tailscale-1";
  const existingConfig = configurations[instanceId];

  useEffect(() => {
    if (existingConfig?.apiKey) {
      setApiToken(existingConfig.apiKey);
    }
  }, [existingConfig]);

  const validateApiToken = async (token: string) => {
    if (!token.startsWith("tskey-api-")) {
      throw new Error("Invalid API key format. Must start with 'tskey-api-'");
    }

    try {
      // Use the existing devices endpoint to validate the token
      const response = await api.get<{ status: string; error?: string }>(
        `/api/tailscale/devices?apiKey=${token}`
      );

      if (response.error) {
        throw new Error(response.error);
      }

      return true;
    } catch (err) {
      console.error("Validation error:", err);
      if (err instanceof Error) {
        throw err;
      }
      throw new Error("Failed to validate API token");
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      if (!isAuthenticated) {
        throw new Error("You must be authenticated to perform this action");
      }

      // Validate the API token using existing endpoint
      await validateApiToken(apiToken);

      // If validation passes, update the configuration
      await updateConfiguration(instanceId, {
        url: "https://api.tailscale.com",
        apiKey: apiToken,
        displayName: "Tailscale",
      });

      // Only refresh the health status for this specific Tailscale instance
      refreshServiceHealth(instanceId);

      toast.success("Tailscale configured successfully");
      handleClose();
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to configure Tailscale";
      toast.error(errorMessage);
      setError(errorMessage);
      console.error("Configuration error:", err);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDelete = async () => {
    if (!instanceId) return;
    setIsDeleting(true);

    try {
      if (!isAuthenticated) {
        throw new Error("You must be authenticated to perform this action");
      }

      await deleteConfiguration(instanceId);
      toast.success("Tailscale configuration removed");
      handleClose();
    } catch (err) {
      const errorMessage =
        err instanceof Error
          ? err.message
          : "Failed to remove Tailscale configuration";
      toast.error(errorMessage);
      console.error("Error removing Tailscale configuration:", err);
    } finally {
      setIsDeleting(false);
    }
  };

  // Reset form state when modal closes
  const handleClose = () => {
    setApiToken(existingConfig?.apiKey || "");
    setError(null);
    onClose();
  };

  const apiKeyHelp = {
    prefix: "Found in ",
    text: "Admin Console > Settings > Keys",
    link: "https://login.tailscale.com/admin/settings/keys",
  };

  return (
    <AnimatedModal
      isOpen={isOpen}
      onClose={handleClose}
      title={configId ? "Configure Tailscale" : "Add Tailscale"}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <FormInput
          id="apiToken"
          label="API Token"
          type="password"
          value={apiToken}
          onChange={(e) => setApiToken(e.target.value)}
          placeholder="Enter your Tailscale API token"
          required
          helpText={apiKeyHelp}
        />

        {error && (
          <div className="text-red-600 dark:text-red-400 text-sm">{error}</div>
        )}

        <div className="flex justify-between">
          {configId && (
            <Button
              variant="danger"
              onClick={handleDelete}
              disabled={isDeleting || !isAuthenticated}
            >
              {isDeleting ? "Removing..." : "Remove"}
            </Button>
          )}
          <div className="flex gap-2 ml-auto">
            <Button
              variant="secondary"
              onClick={handleClose}
              disabled={isSubmitting || isDeleting}
              type="button"
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              type="submit"
              disabled={isSubmitting || isDeleting || !isAuthenticated}
            >
              {isSubmitting ? "Saving..." : configId ? "Update" : "Save"}
            </Button>
          </div>
        </div>
      </form>
    </AnimatedModal>
  );
};
