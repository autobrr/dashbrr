import assert from "node:assert/strict";
import test from "node:test";
import {
  getMediaStatus,
  getRequestStatus,
  OVERSEERR_MEDIA_STATUS,
  OVERSEERR_REQUEST_STATUS,
  resolveRequestStatus,
} from "../src/components/services/overseerr/status.ts";
import type { OverseerrMediaRequest } from "../src/types/service.ts";

const baseRequest = (
  overrides: Partial<OverseerrMediaRequest>
): OverseerrMediaRequest =>
  ({
    id: 1,
    status: 0,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    media: {
      id: 1,
      mediaType: "movie",
      tmdbId: 1,
      status: 1,
      requests: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    requestedBy: {
      id: 1,
      email: "user@example.com",
      username: "user",
      plexToken: "",
      plexUsername: "user",
      userType: 1,
      permissions: 0,
      avatar: "",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      requestCount: 0,
    },
    modifiedBy: {
      id: 1,
      email: "user@example.com",
      username: "user",
      plexToken: "",
      plexUsername: "user",
      userType: 1,
      permissions: 0,
      avatar: "",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      requestCount: 0,
    },
    is4k: false,
    serverId: 1,
    profileId: 1,
    rootFolder: "/data",
    ...overrides,
  }) as OverseerrMediaRequest;

test("parses numeric and symbolic request/media statuses", () => {
  assert.equal(getRequestStatus("3"), 3);
  assert.equal(getRequestStatus("approved"), 2);
  assert.equal(getMediaStatus("partially available"), 4);
  assert.equal(getMediaStatus("5"), 5);
});

test("resolves declined and failed from request status", () => {
  const declined = resolveRequestStatus(
    baseRequest({
      status: 3,
      media: { ...baseRequest({}).media, status: 5 },
    })
  );
  assert.equal(declined.meta.label, "Declined");
  assert.equal(declined.isFallback, false);

  const failed = resolveRequestStatus(
    baseRequest({
      status: "FAILED" as unknown as number,
      media: { ...baseRequest({}).media, status: 5 },
    })
  );
  assert.equal(failed.meta.label, "Failed");
  assert.equal(failed.isFallback, false);
});

test("uses media lifecycle labels when request status is missing", () => {
  const available = resolveRequestStatus(
    baseRequest({
      status: 0,
      media: { ...baseRequest({}).media, status: 5 },
    })
  );
  assert.equal(available.meta.label, "Available");
  assert.equal(available.isFallback, true);

  const requested = resolveRequestStatus(
    baseRequest({
      status: 0,
      media: { ...baseRequest({}).media, status: 1 },
    })
  );
  assert.equal(requested.meta.label, "Requested");
  assert.equal(requested.isFallback, true);
});

test("keeps pending request actions when status is string encoded", () => {
  const pending = resolveRequestStatus(
    baseRequest({
      status: "1" as unknown as number,
      media: { ...baseRequest({}).media, status: 1 },
    })
  );
  assert.equal(pending.requestStatus, OVERSEERR_REQUEST_STATUS.PENDING);
  assert.equal(pending.meta.label, "Pending");
});

test("matches seerr request status enum values", () => {
  assert.deepEqual(OVERSEERR_REQUEST_STATUS, {
    PENDING: 1,
    APPROVED: 2,
    DECLINED: 3,
    FAILED: 4,
    COMPLETED: 5,
  });
});

test("matches seerr media status enum values", () => {
  assert.deepEqual(OVERSEERR_MEDIA_STATUS, {
    UNKNOWN: 1,
    PENDING: 2,
    PROCESSING: 3,
    PARTIALLY_AVAILABLE: 4,
    AVAILABLE: 5,
    BLACKLISTED: 6,
    DELETED: 7,
  });
});

test("prefers media lifecycle for approved requests when media state exists", () => {
  const resolved = resolveRequestStatus(
    baseRequest({
      status: OVERSEERR_REQUEST_STATUS.APPROVED,
      media: { ...baseRequest({}).media, status: OVERSEERR_MEDIA_STATUS.AVAILABLE },
    })
  );

  assert.equal(resolved.meta.label, "Available");
  assert.equal(resolved.isFallback, true);
});
