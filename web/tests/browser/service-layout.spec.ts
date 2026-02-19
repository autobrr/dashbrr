import { expect, test, type Page } from "@playwright/test";

const HARNESS_PATH = "/tests/browser/service-layout-harness.html";

const mountHarness = async (page: Page) => {
  await page.goto(HARNESS_PATH);
  await expect(page.getByTestId("compact-card")).toBeVisible();
  await expect(page.getByTestId("shell-card")).toBeVisible();
};

test("compact card density stays tighter than regular", async ({ page }) => {
  await mountHarness(page);

  const compactFooter = page.getByTestId("compact-footer");
  const regularFooter = page.getByTestId("regular-footer");

  const compactSpacing = await compactFooter.evaluate((element) => {
    const styles = getComputedStyle(element);
    return {
      marginTop: parseFloat(styles.marginTop),
      paddingTop: parseFloat(styles.paddingTop),
    };
  });
  const regularSpacing = await regularFooter.evaluate((element) => {
    const styles = getComputedStyle(element);
    return {
      marginTop: parseFloat(styles.marginTop),
      paddingTop: parseFloat(styles.paddingTop),
    };
  });

  expect(compactSpacing.marginTop).toBeLessThan(regularSpacing.marginTop);
  expect(compactSpacing.paddingTop).toBeLessThan(regularSpacing.paddingTop);

});

test("collapse shell removes idle header gap until expanded", async ({ page }) => {
  await mountHarness(page);

  const shellHeader = page.locator('[data-testid="shell-card"] > div > div').first();
  const shellContent = page.locator('[data-testid="shell-card"] > div > div').nth(1);

  await expect(shellHeader).toHaveClass(/mb-0/);
  await expect(shellContent).toHaveClass(/max-h-0/);

  await shellHeader.click();

  await expect(shellHeader).toHaveClass(/mb-2/);
  await expect(shellContent).toHaveClass(/max-h-\[1000px\]/);
});
