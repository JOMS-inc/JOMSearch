import { test, expect, type Page } from "@playwright/test";

// These tests create users, so they only run against the throwaway test
// database in CI or locally, never against production (no @readonly tag).

// A unique user per test run, so reruns against the same DB don't collide.
function newUser() {
  const id = `${Date.now()}${Math.floor(Math.random() * 1000)}`;
  return {
    username: `e2e_${id}`,
    email: `e2e_${id}@example.com`,
    password: "e2e-Pa55word",
  };
}

async function register(
  page: Page,
  user: ReturnType<typeof newUser>,
  repeat = user.password,
) {
  await page.goto("/register");
  await page.locator('input[name="username"]').fill(user.username);
  await page.locator('input[name="email"]').fill(user.email);
  await page.locator('input[name="password"]').fill(user.password);
  await page.locator('input[name="password2"]').fill(repeat);
  await page.getByRole("button", { name: "Sign Up" }).click();
}

test.describe("authentication", () => {
  test("register, log out and log back in", async ({ page }) => {
    const user = newUser();

    await register(page, user);
    await expect(page).toHaveURL(/\/search$/);
    await expect(page.locator("#nav-logout")).toHaveText(
      `Log out [${user.username}]`,
    );

    await page.locator("#nav-logout").click();
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.locator("#nav-login")).toBeVisible();

    await page.locator('input[name="username"]').fill(user.username);
    await page.locator('input[name="password"]').fill(user.password);
    await page.locator("#login-button").click();
    await expect(page.locator("#nav-logout")).toHaveText(
      `Log out [${user.username}]`,
    );
  });

  test("a logged-in user is sent away from the login page", async ({
    page,
  }) => {
    await register(page, newUser());
    await page.goto("/login");
    await expect(page).toHaveURL(/\/search$/);
  });

  test("mismatched passwords show an error", async ({ page }) => {
    await register(page, newUser(), "something-else");
    await expect(page.locator(".error")).toContainText(
      "Passwords do not match.",
    );
  });

  test("registering the same username twice shows an error", async ({
    page,
    browser,
  }) => {
    const user = newUser();
    await register(page, user);

    // Second attempt in a fresh browser context (not logged in).
    const other = await browser.newPage();
    await register(other, { ...user, email: `other_${user.email}` });
    await expect(other.locator(".error")).toContainText("already taken");
    await other.close();
  });

  test("a wrong password shows an error", async ({ page }) => {
    await page.goto("/login");
    await page.locator('input[name="username"]').fill("admin");
    await page.locator('input[name="password"]').fill("definitely-wrong");
    await page.locator("#login-button").click();
    await expect(page.locator(".error")).toContainText(
      "Invalid username or password.",
    );
  });
});
