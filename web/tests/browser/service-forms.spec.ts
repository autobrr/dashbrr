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

const creates = (page: Page) =>
  page.evaluate(
    () => (window as unknown as { creates: Array<{ type: string; config: Record<string, string> }> }).creates
  );

const serveFailure = (page: Page, httpStatus: number, message: string) =>
  page.route("**/api/health/**", (route) =>
    route.fulfill({ status: httpStatus, json: { status: "error", message } })
  );

const openSonarrAdd = async (page: Page) => {
  await page.goto(HARNESS_PATH);
  await page.getByRole("button", { name: "Add Service" }).click();
  await page.getByRole("menuitem", { name: "Add Sonarr" }).click();
  await page.locator("#url").fill("http://sonarr-new.test:8989");
  await page.locator("#apiKey").fill("new-key");
};

const submitAdd = (page: Page) =>
  page.getByRole("button", { name: "Add Service" }).last().click();

test("adding a service tests it by type, sends only its type, and shows the ID the server picked", async ({ page }) => {
  const tested: string[] = [];
  await page.route("**/api/health/**", (route) => {
    tested.push(new URL(route.request().url()).pathname);
    return route.fulfill({ json: { status: "online" } });
  });
  await openSonarrAdd(page);
  await submitAdd(page);

  await expect.poll(async () => (await creates(page)).length).toBe(1);
  const [create] = await creates(page);
  expect(create.type).toBe("sonarr");
  expect(create.config).not.toHaveProperty("instanceId");
  expect(create.config.url).toBe("http://sonarr-new.test:8989");
  expect(tested).toHaveLength(1);
  expect(tested[0]).toMatch(/\/api\/health\/sonarr$/);
  expect(await saves(page)).toHaveLength(0);

  const ids = page.getByRole("list", { name: "Configured services" });
  await expect(ids.getByText("sonarr-7", { exact: true })).toBeVisible();
  for (const id of EXISTING_IDS) {
    await expect(ids.getByText(id, { exact: true })).toBeVisible();
  }
});

test("an offline service can be renamed without a test", async ({ page }) => {
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

test("the submit button spins while the service is tested", async ({ page }) => {
  let answer = () => {};
  const answered = new Promise<void>((resolve) => (answer = resolve));
  await page.route("**/api/health/**", async (route) => {
    await answered;
    await route.fulfill({ json: { status: "online" } });
  });
  await openSonarrAdd(page);
  await submitAdd(page);

  const submit = page.getByRole("button", { name: "Testing..." });
  await expect(submit).toBeDisabled();
  await expect(submit.locator("svg.animate-spin")).toBeVisible();
  expect(await creates(page)).toHaveLength(0);

  answer();
  await expect.poll(async () => (await creates(page)).length).toBe(1);
});

test("a failed test blocks the save and keeps what the user typed", async ({ page }) => {
  await serveHealth(page, "offline");
  await openSonarrAdd(page);
  await submitAdd(page);

  await expect(page.getByText("Service is offline").first()).toBeVisible();
  await expect(page.locator("#url")).toHaveValue("http://sonarr-new.test:8989");
  await expect(page.locator("#apiKey")).toHaveValue("new-key");
  await expect(page.getByRole("button", { name: "Add Service" }).last()).toBeEnabled();
  expect(await creates(page)).toHaveLength(0);
});

test("a service that does not answer in time blocks the save with a message", async ({ page }) => {
  await serveFailure(page, 504, "The service did not answer within 10s");
  await openSonarrAdd(page);
  await submitAdd(page);

  await expect(page.getByText("The service did not answer within 10s").first()).toBeVisible();
  await expect(page.locator("#apiKey")).toHaveValue("new-key");
  expect(await creates(page)).toHaveLength(0);
});

test("a wrong API key on edit shows the error and keeps the user signed in", async ({ page }) => {
  await serveFailure(page, 502, "Invalid API key");
  await page.goto(`${HARNESS_PATH}?edit=sonarr-2`);
  await page.locator("#apiKey").fill("wrong-key");
  await page.getByRole("button", { name: "Save" }).click();

  await expect(page.getByText("Invalid API key").first()).toBeVisible();
  await expect(page).toHaveURL(/service-forms-harness\.html\?edit=sonarr-2$/);
  await expect(page.locator("#apiKey")).toHaveValue("wrong-key");
  expect(await saves(page)).toHaveLength(0);
});
