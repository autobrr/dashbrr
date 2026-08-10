import { createRoot } from "react-dom/client";

import { UptimeKumaStatsView } from "../../src/components/services/uptimekuma/UptimeKumaStats";
import type { UptimeKumaMonitor } from "../../src/types/service";

const monitors: UptimeKumaMonitor[] = [
  { id: "1", name: "API", type: "http", status: "up", responseTimeMs: 24 },
  { id: "2", name: "Cache", type: "redis", status: "down" },
  { id: "3", name: "Database", type: "tcp", status: "down" },
  { id: "4", name: "Indexer", type: "http", status: "down" },
  { id: "5", name: "Queue", type: "docker", status: "down" },
  { id: "6", name: "Search", type: "http", status: "down" },
  { id: "7", name: "Website", type: "http", status: "down" },
  { id: "8", name: "Worker", type: "docker", status: "pending" },
  { id: "9", name: "Deploy", type: "push", status: "maintenance" },
];

const app = document.getElementById("app");

if (!app) {
  throw new Error("missing app mount");
}

createRoot(app).render(
  <main className="mx-auto max-w-3xl space-y-6">
    <section
      data-testid="primary-card"
      className="@container rounded-lg border border-zinc-700 bg-zinc-800 p-4"
    >
      <UptimeKumaStatsView
        baseURL="https://kuma.example/internal"
        counts={{ total: 9, up: 1, down: 6, pending: 1, maintenance: 1 }}
        summary={{ monitors }}
      />
    </section>
    <section
      data-testid="zero-count-card"
      className="@container rounded-lg border border-zinc-700 bg-zinc-800 p-4"
    >
      <UptimeKumaStatsView
        baseURL={null}
        counts={{ total: 1, up: 1, down: 0, pending: 0, maintenance: 0 }}
        summary={{ monitors: monitors.slice(0, 1) }}
      />
    </section>
    <section
      data-testid="narrow-card"
      className="@container w-72 rounded-lg border border-zinc-700 bg-zinc-800 p-4"
    >
      <UptimeKumaStatsView
        baseURL={null}
        counts={{ total: 9, up: 1, down: 6, pending: 1, maintenance: 1 }}
        summary={{ monitors }}
      />
    </section>
  </main>
);
