/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export type ArrQueueDeleteOptions = {
  removeFromClient: "remove" | "change" | "ignore";
  blocklist: "none" | "blocklist" | "blocklistAndSearch";
};

export const getRemovalMethodText = (
  value: ArrQueueDeleteOptions["removeFromClient"]
) => {
  switch (value) {
    case "remove":
      return "Remove from Download Client";
    case "change":
      return "Change Category";
    case "ignore":
      return "Ignore Download";
  }
};

export const getBlocklistText = (value: ArrQueueDeleteOptions["blocklist"]) => {
  switch (value) {
    case "none":
      return "Do not Blocklist";
    case "blocklistAndSearch":
      return "Blocklist and Search";
    case "blocklist":
      return "Blocklist Only";
  }
};

export const buildArrQueueDeleteQueryParams = (
  instanceId: string,
  opts: ArrQueueDeleteOptions
) => {
  const queryParams = new URLSearchParams();
  queryParams.append("instanceId", instanceId);

  if (opts.removeFromClient === "change") {
    queryParams.append("removeFromClient", "false");
    queryParams.append("changeCategory", "true");
  } else if (opts.removeFromClient === "remove") {
    queryParams.append("removeFromClient", "true");
    queryParams.append("changeCategory", "false");
  } else {
    queryParams.append("removeFromClient", "false");
    queryParams.append("changeCategory", "false");
  }

  if (opts.blocklist === "blocklistAndSearch") {
    queryParams.append("blocklist", "true");
    queryParams.append("skipRedownload", "false");
  } else if (opts.blocklist === "blocklist") {
    queryParams.append("blocklist", "true");
    queryParams.append("skipRedownload", "true");
  } else {
    queryParams.append("blocklist", "false");
    queryParams.append("skipRedownload", "true");
  }

  return queryParams;
};

