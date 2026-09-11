import { expect, test } from "@playwright/test";

const HARNESS_PATH = "/tests/browser/health-fields-harness.html";

// Regression test for CustomServiceHealthFields' okValues/warnValues inputs:
// deriving the displayed value straight from the parsed array re-rendered
// the input on every keystroke, so a trailing "," (needed to start a second
// value) was immediately stripped and the user could never type past the
// first value. The fix keeps raw text in local state and only commits the
// parsed array on blur.
test("typing a trailing comma is not dropped while the OK values input is focused", async ({
  page,
}) => {
  await page.goto(HARNESS_PATH);

  const okInput = page.getByLabel("OK values (comma separated)");
  await okInput.click();
  await okInput.pressSequentially("ok,", { delay: 20 });

  // The comma must still be there while focused - this is the bug: the old
  // implementation derived the value from the parsed array on every
  // keystroke and immediately dropped it.
  await expect(okInput).toHaveValue("ok,");

  await okInput.pressSequentially(" healthy", { delay: 20 });
  await expect(okInput).toHaveValue("ok, healthy");

  // Nothing is committed to the parent's config until blur.
  await expect(page.getByTestId("committed-ok-values")).toHaveText("[]");

  await okInput.blur();

  await expect(page.getByTestId("committed-ok-values")).toHaveText(
    "[\"ok\",\"healthy\"]"
  );
  // After blur the input re-syncs from the committed (trimmed) array.
  await expect(okInput).toHaveValue("ok, healthy");
});

// Regression test: commit-on-blur alone leaves a gap - pressing Enter to
// submit the form while focused in this input fires the submit handler
// (reading `health` from parent state) before blur ever runs, so the
// just-typed, uncommitted text is dropped. The fix also commits on Enter
// keydown, without preventDefault, so Enter still submits the form.
test("pressing Enter commits the value without needing to blur first", async ({
  page,
}) => {
  await page.goto(HARNESS_PATH);

  const okInput = page.getByLabel("OK values (comma separated)");
  await okInput.click();
  await okInput.pressSequentially("a,b", { delay: 20 });
  await expect(okInput).toHaveValue("a,b");

  // Still uncommitted at this point.
  await expect(page.getByTestId("committed-ok-values")).toHaveText("[]");

  await okInput.press("Enter");

  await expect(page.getByTestId("committed-ok-values")).toHaveText(
    "[\"a\",\"b\"]"
  );
});

test("the warning values input behaves the same way and is independent of OK values", async ({
  page,
}) => {
  await page.goto(HARNESS_PATH);

  const warnInput = page.getByLabel("Warning values (comma separated)");
  await warnInput.click();
  await warnInput.pressSequentially("degraded,", { delay: 20 });
  await expect(warnInput).toHaveValue("degraded,");

  await warnInput.blur();
  await expect(page.getByTestId("committed-warn-values")).toHaveText(
    "[\"degraded\"]"
  );
  await expect(page.getByTestId("committed-ok-values")).toHaveText("[]");
});
