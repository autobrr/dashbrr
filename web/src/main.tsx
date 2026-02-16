/*
 * Copyright (c) 2024, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";

// Force dark mode
document.documentElement.classList.add("dark");

function unregisterServiceWorkerInDev() {
  if (!import.meta.env.DEV) return;
  if (!("serviceWorker" in navigator)) return;

  // Fix dev state where a previously-registered SW is still controlling the page
  // and serves stale/raw CSS (e.g. `@tailwind` directives) from cache.
  navigator.serviceWorker
    .getRegistrations()
    .then((regs) => Promise.all(regs.map((r) => r.unregister())))
    .catch(() => {});

  if ("caches" in window) {
    caches
      .keys()
      .then((keys) => Promise.all(keys.map((k) => caches.delete(k))))
      .catch(() => {});
  }
}

unregisterServiceWorkerInDev();

if (import.meta.env.PROD) {
  // Register service worker with auto update handling
  import("virtual:pwa-register")
    .then(({ registerSW }) => {
      const updateSW = registerSW({
        onNeedRefresh() {
          if (confirm("New version available! Click OK to update.")) {
            updateSW(true);
          }
        },
        onOfflineReady() {
          console.log("App ready to work offline");
        },
        immediate: true,
      });
    })
    .catch(() => {});
}

const root = document.getElementById("root");
if (!root) {
  throw new Error("Root element not found");
}

ReactDOM.createRoot(root).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
