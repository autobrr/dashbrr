/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { toast } from "react-hot-toast";

import { editServiceSetup } from "../../config/serviceSetup";
import { useConfiguration } from "../../contexts/useConfiguration";
import { ServiceConfigInput } from "../../types/service";
import { ServiceSetupForm } from "./ServiceSetupForm";

interface ConfigurationFormProps {
  instanceId: string;
  onClose: () => void;
}

export const ConfigurationForm = ({
  instanceId,
  onClose,
}: ConfigurationFormProps) => {
  const { configurations, updateConfiguration } = useConfiguration();

  const save = async (input: ServiceConfigInput) => {
    await updateConfiguration(instanceId, input);
    toast.success("Configuration saved successfully");
    onClose();
  };

  return (
    <ServiceSetupForm
      setup={editServiceSetup(instanceId, configurations)}
      submitLabel="Save"
      onSubmit={save}
      onCancel={onClose}
    />
  );
};
