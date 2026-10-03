import assert from "node:assert/strict";
import test from "node:test";
import { loginTypeFrom } from "../src/utils/loginType.ts";

test("loginTypeFrom reads the login type from the verify response", () => {
  assert.equal(loginTypeFrom({ auth_type: "builtin", user_id: 1 }), "builtin");
  assert.equal(loginTypeFrom({ auth_type: "oidc", user_id: 0 }), "oidc");
});

test("loginTypeFrom returns null when auth_type is missing or unknown", () => {
  assert.equal(loginTypeFrom({ message: "Token is valid" }), null);
  assert.equal(loginTypeFrom({ auth_type: "plex" }), null);
  assert.equal(loginTypeFrom(null), null);
});
