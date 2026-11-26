import { expect, test } from "@playwright/test";

const HARNESS_PATH = "/tests/browser/mesh-network-harness.html";

test("only Tailscale configured shows one Tailscale button", async ({ page }) => {
  await page.goto(`${HARNESS_PATH}?scenario=tailscale-only`);
  const header = page.getByTestId("header-tailscale_only");

  await expect(header.getByTestId("tailscale-button")).toBeVisible();
  await expect(header.getByTestId("tailscale-button")).toContainText("Tailscale:Connected");
  await expect(header.getByTestId("netbird-button")).toHaveCount(0);
});

test("only NetBird configured shows one NetBird button with peers", async ({ page }) => {
  await page.goto(`${HARNESS_PATH}?scenario=netbird-only`);
  const header = page.getByTestId("header-netbird_only");

  await expect(header.getByTestId("netbird-button")).toBeVisible();
  await expect(header.getByTestId("netbird-button")).toContainText("1/2 peers online");
  await expect(header.getByTestId("netbird-button")).toContainText("NetBird");
  await expect(header.getByTestId("tailscale-button")).toHaveCount(0);
  await expect(header.getByText("Update available")).toBeVisible();
});

test("both networks show two buttons in Tailscale-first order", async ({ page }) => {
  await page.goto(`${HARNESS_PATH}?scenario=both`);
  const header = page.getByTestId("header-both");

  const buttons = header.getByTestId(/^(tailscale|netbird)-button$/);
  await expect(buttons).toHaveCount(2);
  const ids = await buttons.evaluateAll((nodes) =>
    nodes.map((node) => node.getAttribute("data-testid"))
  );
  expect(ids).toEqual(["tailscale-button", "netbird-button"]);

  // Both buttons are in the same header row. On a phone, the longer label wraps
  // to two lines, so the buttons share a vertical center, not a top.
  const centers = await buttons.evaluateAll((nodes) =>
    nodes.map((node) => {
      const box = node.getBoundingClientRect();
      return box.top + box.height / 2;
    })
  );
  expect(centers[0]).toBe(centers[1]);
});

test("the NetBird list opens from its own button", async ({ page }) => {
  await page.goto(`${HARNESS_PATH}?scenario=both`);
  const header = page.getByTestId("header-both");

  await header.getByTestId("netbird-button").click();

  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("NetBird Peers")).toBeVisible();
  await expect(dialog.getByText("1 of 2 online")).toBeVisible();
  await expect(dialog.getByPlaceholder("Search peers by name, OS, IP, group, or city...")).toBeVisible();

  // The update badge sorts the outdated peer to the top, and the "All" group stays out.
  const rows = dialog.locator(".space-y-2 > div");
  await expect(rows.first()).toContainText("host-1");
  await expect(dialog.getByText("homelab")).toBeVisible();
  await expect(dialog.getByText("Login expired")).toBeVisible();
  await expect(rows.first().getByRole("link", { name: "Update Available" })).toHaveAttribute(
    "href",
    "https://github.com/netbirdio/netbird/releases"
  );
});

test("a token that sees no peers shows the hint on the NetBird button", async ({ page }) => {
  await page.goto(`${HARNESS_PATH}?scenario=netbird-warning`);
  const button = page.getByTestId("header-netbird_warning").getByTestId("netbird-button");

  await expect(button).toContainText("Token sees no peers: use a token from an admin user");
});

test("a discovered network has no configure and no remove button", async ({ page }) => {
  await page.goto(`${HARNESS_PATH}?scenario=discovered`);
  const header = page.getByTestId("header-discovered");

  await expect(header.getByTestId("netbird-button")).toBeVisible();
  await expect(header.getByTestId("netbird-configure")).toHaveCount(0);
  await expect(header.getByTitle("Remove NetBird")).toHaveCount(0);
});

test("the Tailscale list keeps its fields and the word devices", async ({ page }) => {
  await page.goto(`${HARNESS_PATH}?scenario=both`);
  const header = page.getByTestId("header-both");

  await header.getByTestId("tailscale-button").click();

  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Tailscale Devices")).toBeVisible();
  await expect(dialog.getByPlaceholder("Search devices by name, type, IP, or tags...")).toBeVisible();
  await expect(dialog.getByText("100.64.0.7")).toBeVisible();
  await expect(dialog.getByText("tag:work")).toBeVisible();
  await expect(dialog.getByText(/Tailscale Peers/)).toHaveCount(0);
  // An outdated device gets only the changelog link, the same as on develop. It
  // gets no other badge.
  await expect(dialog.getByRole("link", { name: "Update Available" })).toHaveCount(1);
  await expect(dialog.getByText("Update", { exact: true })).toHaveCount(0);
});

test("the header offers a configure path next to the remove button", async ({ page }) => {
  await page.goto(`${HARNESS_PATH}?scenario=netbird-only`);
  const header = page.getByTestId("header-netbird_only");

  const configure = header.getByTestId("netbird-configure");
  await expect(configure).toBeVisible();
  await expect(configure).toHaveAttribute("title", "Configure NetBird");

  await configure.click();

  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Configure NetBird")).toBeVisible();
  await expect(dialog.locator("#url")).toHaveValue("https://api.netbird.io");
});
