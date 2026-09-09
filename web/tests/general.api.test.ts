import assert from "node:assert/strict";
import test from "node:test";

import { runGeneralAction } from "../src/api/general.ts";

// Regression tests: runGeneralAction used to build its own request with a
// raw `fetch` call, bypassing the shared `api` helper's session-expiry
// redirect, 429 retry/backoff, and timeout handling. It must now go through
// `api.post`, which prefixes `/api` when the path lacks it and merges in
// the extra X-Confirm header when needed.

test("runGeneralAction sends X-Confirm when confirm=true", async (t) => {
  let capturedInput: string | URL | Request | undefined;
  let capturedInit: RequestInit | undefined;

  t.mock.method(globalThis, "fetch", async (input: string | URL | Request, init?: RequestInit) => {
    capturedInput = input;
    capturedInit = init;
    return new Response(JSON.stringify({ status: 200, body: "ok" }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  });

  const result = await runGeneralAction("general-1", "restart", true);

  assert.equal(capturedInput, "/api/general/general-1/actions/restart");
  const headers = new Headers(capturedInit?.headers);
  assert.equal(headers.get("X-Confirm"), "yes");
  assert.equal(capturedInit?.method, "POST");
  assert.deepEqual(result, { status: 200, body: "ok" });
});

test("runGeneralAction omits X-Confirm when confirm=false", async (t) => {
  let capturedInit: RequestInit | undefined;

  t.mock.method(globalThis, "fetch", async (_input: string | URL | Request, init?: RequestInit) => {
    capturedInit = init;
    return new Response(JSON.stringify({ status: 200 }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  });

  await runGeneralAction("general-1", "pause-all", false);

  const headers = new Headers(capturedInit?.headers);
  assert.equal(headers.get("X-Confirm"), null);
});

test("runGeneralAction surfaces the server error message on failure", async (t) => {
  t.mock.method(globalThis, "fetch", async () => {
    return new Response(JSON.stringify({ error: "this action requires confirmation" }), {
      status: 428,
      headers: { "content-type": "application/json" },
    });
  });

  await assert.rejects(
    () => runGeneralAction("general-1", "restart", false),
    (err: Error) => {
      assert.match(err.message, /confirmation/);
      return true;
    }
  );
});
