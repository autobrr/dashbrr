import assert from "node:assert/strict";
import test from "node:test";
import { requestLink } from "../src/components/services/overseerr/requestLink.ts";

const service = {
  url: "http://overseerr.overseerr.svc.cluster.local",
  accessUrl: "https://overseerr.example.test",
};

test("requestLink uses serviceUrl when Overseerr sets it", () => {
  assert.equal(
    requestLink(service, {
      mediaType: "movie",
      tmdbId: 977942,
      serviceUrl: "https://radarr.example.test/movie/977942",
    }),
    "https://radarr.example.test/movie/977942"
  );
});

test("requestLink falls back to the Overseerr movie page", () => {
  assert.equal(
    requestLink(service, { mediaType: "movie", tmdbId: 977942, serviceUrl: "" }),
    "https://overseerr.example.test/movie/977942"
  );
});

test("requestLink falls back to the Overseerr TV page", () => {
  assert.equal(
    requestLink(service, { mediaType: "tv", tmdbId: 1399, serviceUrl: "" }),
    "https://overseerr.example.test/tv/1399"
  );
});

test("requestLink returns null without a TMDB ID or a valid service URL", () => {
  assert.equal(requestLink(service, { mediaType: "movie", serviceUrl: "" }), null);
  assert.equal(requestLink(service, { mediaType: "movie", tmdbId: 0 }), null);
  assert.equal(
    requestLink({ url: "not a url" }, { mediaType: "movie", tmdbId: 1 }),
    null
  );
});
