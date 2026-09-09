// Standalone Playwright test harness page, not part of the app's module
// graph - not a fast-refresh boundary.
/* eslint-disable react-refresh/only-export-components */
import { useState } from "react";
import { createRoot } from "react-dom/client";

import { CustomServiceHealthFields } from "../../src/components/configuration/general/CustomServiceHealthFields";
import type { CustomHealthConfig } from "../../src/types/service";

const Harness = () => {
  const [health, setHealth] = useState<CustomHealthConfig>({ path: "/health" });

  return (
    <main className="mx-auto max-w-md space-y-4">
      <section
        data-testid="fields"
        className="rounded-lg border border-zinc-700 bg-zinc-800 p-4"
      >
        <CustomServiceHealthFields health={health} onChange={setHealth} />
      </section>
      <pre data-testid="committed-ok-values">
        {JSON.stringify(health.okValues ?? [])}
      </pre>
      <pre data-testid="committed-warn-values">
        {JSON.stringify(health.warnValues ?? [])}
      </pre>
    </main>
  );
};

const app = document.getElementById("app");

if (!app) {
  throw new Error("missing app mount");
}

createRoot(app).render(<Harness />);
