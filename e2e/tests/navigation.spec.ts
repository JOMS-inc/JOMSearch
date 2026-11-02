import { test, expect } from "@playwright/test";

// @readonly: these tests only read, so they also run against production after deploy.

test.describe("navigation and layout @readonly", () => {
  test("nav links go to the login and register pages", async ({ page }) => {
    await page.goto("/");

    await page.locator("#nav-login").click();
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByRole("heading", { name: "Log In" })).toBeVisible();

    await page.locator("#nav-register").click();
    await expect(page).toHaveURL(/\/register$/);
    await expect(page.getByRole("heading", { name: "Sign Up" })).toBeVisible();

    await page.locator("#nav-logo").click();
    await expect(page).toHaveURL(/\/search$/);
  });

  test("footer About link shows the about page with the team image", async ({
    page,
  }) => {
    await page.goto("/");
    await page.locator(".footer").getByRole("link", { name: "About" }).click();

    await expect(page).toHaveURL(/\/about$/);
    await expect(
      page.getByRole("heading", { name: "Our mission" }),
    ).toBeVisible();

    // The image must actually load (naturalWidth is 0 for a broken image).
    const img = page.getByAltText("monk group our team");
    await expect(img).toBeVisible();
    expect(
      await img.evaluate((el: HTMLImageElement) => el.naturalWidth),
    ).toBeGreaterThan(0);
  });

  test("the stylesheet loads", async ({ page }) => {
    const response = await page.request.get("/static/style.css");
    expect(response.status()).toBe(200);
    expect(response.headers()["content-type"]).toContain("text/css");
  });

  test("the footer sits at the bottom of a short page", async ({ page }) => {
    await page.goto("/login");
    const viewport = page.viewportSize()!;
    const footer = await page.locator(".footer").boundingBox();

    expect(footer).not.toBeNull();
    // Bottom edge of the footer lines up with the bottom of the window (±1px rounding).
    expect(
      Math.abs(footer!.y + footer!.height - viewport.height),
    ).toBeLessThanOrEqual(1);
  });
});
