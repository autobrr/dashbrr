import { expect, test } from "@playwright/test";

const HARNESS_PATH = "/tests/browser/uptimekuma-card-harness.html";

test("issues auto-open and every row stays reachable", async ({ page }) => {
  await page.goto(HARNESS_PATH);
  const card = page.getByTestId("primary-card");

  await expect(card.getByRole("heading", { name: "Needs Attention" })).toBeVisible();
  await expect(card.getByRole("link", { name: /Open .* in Uptime Kuma/ })).toHaveCount(7);
  const cacheLink = card.getByRole("link", {
    name: "Open Cache, redis, Down in Uptime Kuma",
  });
  await expect(cacheLink).toHaveAttribute(
    "href",
    "https://kuma.example/internal/dashboard/2"
  );
  await expect(cacheLink).toHaveAttribute("target", "_blank");
  await expect(cacheLink).toHaveAttribute("rel", "noopener noreferrer");
  await expect(card.getByText("7 monitors")).toBeVisible();
  await expect(card.getByRole("link", { name: "Open Uptime Kuma" })).toHaveAttribute(
    "href",
    "https://kuma.example/internal/dashboard"
  );
  await expect(card.getByText("API", { exact: true })).toHaveCount(0);

  const worker = card.getByRole("link", {
    name: "Open Worker, docker, Pending in Uptime Kuma",
  });
  await worker.scrollIntoViewIfNeeded();
  await expect(worker).toBeVisible();
});

test("all nonzero stat tiles filter the monitor list", async ({ page }) => {
  await page.goto(HARNESS_PATH);
  const card = page.getByTestId("primary-card");

  const cases = [
    { tile: "Total 9", heading: "All Monitors", visibleLink: /Open API,.*Up.*in Uptime Kuma/ },
    { tile: "Up 1", heading: "Up Monitors", visibleLink: /Open API,.*Up.*in Uptime Kuma/ },
    { tile: "Down 6", heading: "Down Monitors", visibleLink: /Open Cache,.*Down.*in Uptime Kuma/ },
    { tile: "Pending 1", heading: "Pending Monitors", visibleLink: /Open Worker,.*Pending.*in Uptime Kuma/ },
    {
      tile: "Maintenance 1",
      heading: "Maintenance Monitors",
      visibleLink: /Open Deploy,.*Maintenance.*in Uptime Kuma/,
    },
  ];

  for (const { tile, heading, visibleLink } of cases) {
    const button = card.getByRole("button", { name: tile });
    await button.click();
    await expect(button).toHaveAttribute("aria-pressed", "true");
    await expect(card.getByRole("heading", { name: heading })).toBeVisible();
    await expect(card.getByRole("link", { name: visibleLink })).toBeVisible();
  }
});

test("clicking the active filter returns to needs attention", async ({ page }) => {
  await page.goto(HARNESS_PATH);
  const card = page.getByTestId("primary-card");
  const upButton = card.getByRole("button", { name: "Up 1" });

  await upButton.click();
  await expect(card.getByRole("heading", { name: "Up Monitors" })).toBeVisible();

  await upButton.click();
  await expect(upButton).toHaveAttribute("aria-pressed", "false");
  await expect(
    card.getByRole("heading", { name: "Needs Attention" })
  ).toBeVisible();
});

test("zero-count stat tiles are disabled", async ({ page }) => {
  await page.goto(HARNESS_PATH);

  const card = page.getByTestId("zero-count-card");
  await expect(card.getByRole("button", { name: "Down 0" })).toBeDisabled();
  await expect(card.getByRole("button", { name: "Pending 0" })).toBeDisabled();
  await expect(card.getByRole("button", { name: "Maintenance 0" })).toBeDisabled();
});

test("monitors remain readable when no safe Uptime Kuma URL exists", async ({ page }) => {
  await page.goto(HARNESS_PATH);
  const card = page.getByTestId("zero-count-card");

  await card.getByRole("button", { name: "Up 1" }).click();
  await expect(card.getByText("API", { exact: true })).toBeVisible();
  await expect(card.getByRole("link")).toHaveCount(0);
});

test("each monitor list has a unique accessible label", async ({ page }) => {
  await page.goto(HARNESS_PATH);
  await page
    .getByTestId("zero-count-card")
    .getByRole("button", { name: "Up 1" })
    .click();

  const labelledSections = await page
    .locator("section[aria-labelledby]")
    .evaluateAll((sections) =>
      sections.map((section) => section.getAttribute("aria-labelledby"))
    );

  expect(labelledSections).toHaveLength(3);
  expect(new Set(labelledSections).size).toBe(3);
});

test("stat tile labels stay readable in a narrow card", async ({ page }) => {
  await page.goto(HARNESS_PATH);

  const label = page
    .getByTestId("narrow-card")
    .getByRole("button", { name: "Maintenance 1" })
    .locator("div")
    .first();

  // The tile grid keys off the card width, not the viewport, so a narrow card
  // drops to two columns instead of clipping the longest label.
  const overflow = await label.evaluate(
    (node) => node.scrollWidth - node.clientWidth
  );
  expect(overflow).toBe(0);
});
