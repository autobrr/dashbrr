import assert from "node:assert/strict";
import test from "node:test";
import { canRemoveQueueItem } from "../src/components/services/common/ArrQueueDelete.ts";

test("queue items in importBlocked or importPending can be removed", () => {
  assert.equal(canRemoveQueueItem({ trackedDownloadState: "importBlocked" }), true);
  assert.equal(canRemoveQueueItem({ trackedDownloadState: "importPending" }), true);
});

test("queue items in other states cannot be removed", () => {
  for (const state of ["downloading", "importing", "imported", "failedPending", "failed", "ignored", ""]) {
    assert.equal(canRemoveQueueItem({ trackedDownloadState: state }), false, state);
  }
});
