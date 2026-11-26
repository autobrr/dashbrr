import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { serviceTemplates } from "../src/config/serviceTemplates.ts";

test("the add menu offers the same service templates in the same order", () => {
  const expected = JSON.parse(
    readFileSync(new URL("./fixtures/serviceTemplates.json", import.meta.url), "utf8")
  );
  assert.deepStrictEqual(JSON.parse(JSON.stringify(serviceTemplates)), expected);
});
