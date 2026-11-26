import { expect, test, type Page } from "@playwright/test";

const HARNESS_PATH = "/tests/browser/service-forms-harness.html";
const EXISTING_IDS = ["sonarr-2", "tailscale-1", "netbird-1", "general-1"];

const serveHealth = (page: Page, status: string) =>
  page.route("**/api/health/**", (route) =>
    route.fulfill({ json: { status, message: `Service is ${status}` } })
  );

const saves = (page: Page) =>
  page.evaluate(
    () => (window as unknown as { saves: Array<{ instanceId: string; config: Record<string, string> }> }).saves
  );

test("adding a service takes an instance ID that no service has", async ({ page }) => {
  await page.goto(HARNESS_PATH);
  await page.getByRole("button", { name: "Add Service" }).click();
  await page.getByRole("menuitem", { name: "Add Sonarr" }).click();
  await page.locator("#url").fill("http://sonarr-new.test:8989");
  await page.locator("#apiKey").fill("new-key");
  await page.getByRole("button", { name: "Add Service" }).last().click();

  await expect.poll(async () => (await saves(page)).length).toBe(1);
  const [save] = await saves(page);
  expect(EXISTING_IDS).not.toContain(save.instanceId);
  expect(save.instanceId).toMatch(/^sonarr-/);
});

test("an offline service can be edited", async ({ page }) => {
  await serveHealth(page, "offline");
  await page.goto(`${HARNESS_PATH}?edit=sonarr-2`);
  await page.locator("#displayName").fill("Sonarr 4K");
  await page.getByRole("button", { name: "Save" }).click();

  await expect.poll(async () => (await saves(page)).length).toBe(1);
  const [save] = await saves(page);
  expect(save.instanceId).toBe("sonarr-2");
  expect(save.config.displayName).toBe("Sonarr 4K");
});

test("a General service's API key can be changed", async ({ page }) => {
  await serveHealth(page, "online");
  await page.goto(`${HARNESS_PATH}?edit=general-1`);
  await page.locator("#apiKey").fill("status-token");
  await page.getByRole("button", { name: "Save" }).click();

  await expect.poll(async () => (await saves(page)).length).toBe(1);
  const [save] = await saves(page);
  expect(save.config.apiKey).toBe("status-token");
});

test("editing Tailscale asks for no URL, as adding it does", async ({ page }) => {
  await serveHealth(page, "online");
  await page.goto(`${HARNESS_PATH}?edit=tailscale-1`);
  await expect(page.locator("#displayName")).toBeVisible();
  await expect(page.locator("#url")).toHaveCount(0);

  await page.locator("#apiKey").fill("tskey-api-new");
  await page.getByRole("button", { name: "Save" }).click();

  await expect.poll(async () => (await saves(page)).length).toBe(1);
  const [save] = await saves(page);
  expect(save.config.url).toBe("https://api.tailscale.com");
});
