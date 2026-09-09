// Standalone Playwright test harness page, not part of the app's module
// graph - not a fast-refresh boundary.
/* eslint-disable react-refresh/only-export-components */
import { useRef, useState } from "react";
import { createRoot } from "react-dom/client";

import { GeneralStatsView } from "../../src/components/services/general/GeneralStats";
import type { GeneralActionResult } from "../../src/api/general";
import type { Service } from "../../src/types/service";

const service: Service = {
  id: "general-1",
  instanceId: "general-1",
  name: "General Service",
  displayName: "My API",
  type: "general",
  status: "online",
  url: "https://api.example.com",
  stats: {
    general: {
      stats: {},
      actions: [
        { id: "restart", label: "Restart" },
        { id: "pause-all", label: "Pause All", confirm: true },
      ],
    },
  },
};

const Harness = () => {
  const [callCount, setCallCount] = useState(0);
  const callsRef = useRef<string[]>([]);

  const runGeneralActionFn = async (
    instanceId: string,
    actionId: string,
    confirm: boolean
  ): Promise<GeneralActionResult> => {
    callsRef.current.push(`${actionId}:${confirm}`);
    setCallCount(callsRef.current.length);
    // Controllable delay so a test can click again while the first call is
    // still in flight, to exercise the overlapping-request guard.
    await new Promise((resolve) => setTimeout(resolve, 300));
    return { status: 200, body: "ok" };
  };

  return (
    <main className="mx-auto max-w-md space-y-4">
      <section
        data-testid="general-stats"
        className="rounded-lg border border-zinc-700 bg-zinc-800 p-4"
      >
        <GeneralStatsView service={service} runGeneralActionFn={runGeneralActionFn} />
      </section>
      <div data-testid="call-count">{callCount}</div>
    </main>
  );
};

const app = document.getElementById("app");

if (!app) {
  throw new Error("missing app mount");
}

createRoot(app).render(<Harness />);
