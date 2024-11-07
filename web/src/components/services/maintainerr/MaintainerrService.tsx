/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import { MaintainerrCollections } from "./MaintainerrCollections";

interface MaintainerrServiceProps {
  instanceId: string;
}

export const MaintainerrService: React.FC<MaintainerrServiceProps> = ({
  instanceId,
}) => {
  return <MaintainerrCollections instanceId={instanceId} />;
};
