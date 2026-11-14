import { expect, test } from "@playwright/test";

const HARNESS_PATH = "/tests/browser/mesh-network-harness.html";

test("only Tailscale configured shows one Tailscale button", async ({ page }) => {
  await page.goto(`${HARNESS_PATH}?scenario=tailscale-only`);
  const header = page.getByTestId("header-tailscale_only");

  await expect(header.getByTestId("tailscale-button")).toBeVisible();
  await expect(header.getByTestId("tailscale-button")).toContainText("1/2 devices online");
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

  // The grid keeps the mesh VPNs out of the header row when both are set up.
  const boxes = await buttons.evaluateAll((nodes) =>
    nodes.map((node) => node.getBoundingClientRect().top)
  );
  expect(boxes[0]).toBe(boxes[1]);
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
