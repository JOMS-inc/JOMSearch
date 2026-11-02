import { test, expect } from "@playwright/test";

// @readonly: these tests only read, so they also run against production after deploy.
// "programming" has results both in the seeded test DB and in production.

test.describe("search @readonly", () => {
  test("the search box is focused on load", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("#search-input")).toBeFocused();
  });

  test("pressing Enter searches and shows results", async ({ page }) => {
    await page.goto("/");
    await page.locator("#search-input").fill("programming");
    await page.keyboard.press("Enter");

    await expect(page).toHaveURL(/[?&]q=programming/);
    await expect(page.locator("#search-input")).toHaveValue("programming");
    await expect(page.locator(".search-result-title").first()).toBeVisible();
  });

  test("clicking the Search button searches too", async ({ page }) => {
    await page.goto("/");
    await page.locator("#search-input").fill("programming");
    await page.locator("#search-button").click();

    await expect(page).toHaveURL(/[?&]q=programming/);
    await expect(page.locator(".search-result-title").first()).toBeVisible();
  });

  test("a nonsense query shows no results", async ({ page }) => {
    await page.goto("/?q=zzqqxxnotarealword");
    await expect(page.locator(".search-result-title")).toHaveCount(0);
  });
});
