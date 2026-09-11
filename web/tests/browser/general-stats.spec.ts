import { expect, test } from "@playwright/test";

const HARNESS_PATH = "/tests/browser/general-stats-harness.html";

// Regression test: GeneralStats.runAction started a request whenever
// called, even if another action was already pending - and the confirm
// dialog's Confirm button stayed enabled while a request was in flight -
// so a fast double-click (or clicking a second action while the first was
// still running) could fire duplicate custom-action POSTs. The fix returns
// early when an action is already running and disables every action
// control, including Confirm, until the request settles.
test("a fast double-click on a non-confirm action only fires one request", async ({
  page,
}) => {
  await page.goto(HARNESS_PATH);
  const card = page.getByTestId("general-stats");

  const restart = card.getByRole("button", { name: /^Restart$|^Running\.\.\.$/ });
  await restart.click();
  // Second click while the first request (300ms) is still in flight.
  await restart.click({ force: true });

  await expect(page.getByTestId("call-count")).toHaveText("1");
  await expect(card.getByRole("button", { name: "Running..." })).toBeVisible();

  // Wait for the in-flight request to settle and confirm no extra call
  // fired afterwards.
  await page.waitForTimeout(400);
  await expect(page.getByTestId("call-count")).toHaveText("1");
  await expect(card.getByRole("button", { name: "Restart" })).toBeEnabled();
});

test("every action control is disabled while one action is running", async ({
  page,
}) => {
  await page.goto(HARNESS_PATH);
  const card = page.getByTestId("general-stats");

  await card.getByRole("button", { name: "Restart" }).click();

  // The OTHER action's button must also be disabled while Restart runs -
  // not just the one that was clicked.
  await expect(card.getByRole("button", { name: "Pause All" })).toBeDisabled();

  await page.waitForTimeout(400);
  await expect(card.getByRole("button", { name: "Pause All" })).toBeEnabled();
});

test("the confirm dialog's Confirm button is disabled while the request runs, and repeated clicks only fire once", async ({
  page,
}) => {
  await page.goto(HARNESS_PATH);
  const card = page.getByTestId("general-stats");

  await card.getByRole("button", { name: "Pause All" }).click();

  const dialog = page.getByLabel("Confirm action");
  const confirmButton = dialog.getByRole("button", { name: /^Confirm$|^Running\.\.\.$/ });
  await expect(confirmButton).toBeVisible();
  await confirmButton.click();

  // Still-visible dialog button must now read "Running..." and be
  // disabled, and clicking it again must not fire a second request.
  const runningInDialog = dialog.getByRole("button", { name: "Running..." });
  await expect(runningInDialog).toBeDisabled();
  await runningInDialog.click({ force: true });

  await expect(page.getByTestId("call-count")).toHaveText("1");

  await page.waitForTimeout(400);
  await expect(page.getByTestId("call-count")).toHaveText("1");
});
