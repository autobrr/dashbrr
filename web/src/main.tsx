/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./i18n";
import "./index.css";

// Force dark mode
document.documentElement.classList.add("dark");

async function unregisterServiceWorkerInDev() {
  if (!import.meta.env.DEV) return;
  if (!("serviceWorker" in navigator)) return;

  const wasControlled = !!navigator.serviceWorker.controller;

  // Fix dev state where a previously-registered SW is still controlling the page
  // and serves stale/raw CSS (e.g. `@tailwind` directives) from cache.
  try {
    const regs = await navigator.serviceWorker.getRegistrations();
    await Promise.all(regs.map((r) => r.unregister()));
  } catch {
    // Ignore: best-effort dev cleanup.
  }

  if ("caches" in window) {
    try {
      const keys = await caches.keys();
      await Promise.all(keys.map((k) => caches.delete(k)));
    } catch {
      // Ignore: best-effort dev cleanup.
    }
  }

  // If a SW was controlling the page, this load can still be stale.
  // Reload once (per tab) after unregistering + cache purge.
  if (wasControlled && !sessionStorage.getItem("dashbrr_dev_sw_killed")) {
    sessionStorage.setItem("dashbrr_dev_sw_killed", "1");
    location.reload();
  }
}

void unregisterServiceWorkerInDev();

const root = document.getElementById("root");
if (!root) {
  throw new Error("Root element not found");
}

ReactDOM.createRoot(root).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
