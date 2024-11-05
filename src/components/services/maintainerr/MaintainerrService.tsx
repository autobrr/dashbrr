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
