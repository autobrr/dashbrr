/*
 * Copyright (c) 2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import { testService } from "../src/utils/serviceTest";

const realFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = realFetch;
});

const recordFetch = () => {
  const calls: { url: string; init?: RequestInit }[] = [];
  globalThis.fetch = (async (url: string, init?: RequestInit) => {
    calls.push({ url, init });
    return new Response(JSON.stringify({ status: "online" }), {
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
  return calls;
};

test("the service test sends the URL and key in the body, not the request URL", async () => {
  const calls = recordFetch();

  await testService("nzbget-1", {
    url: "http://user:pass@nzbget:6789",
    apiKey: "SECRET-KEY-123",
  });

  assert.equal(calls.length, 1);
  const [{ url, init }] = calls;
  assert.equal(url, "api/health/nzbget-1");
  assert.equal(init?.method, "POST");
  assert.deepEqual(JSON.parse(String(init?.body)), {
    url: "http://user:pass@nzbget:6789",
    apiKey: "SECRET-KEY-123",
  });
});

test("the service test leaves out a blank key so the server can use the stored one", async () => {
  const calls = recordFetch();

  await testService("sonarr-1", { url: "http://sonarr:8989", apiKey: "" });

  assert.deepEqual(JSON.parse(String(calls[0].init?.body)), { url: "http://sonarr:8989" });
});
