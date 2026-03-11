import { Page } from '@playwright/test'

/**
 * loginAsDev navigates to the dashboard and handles the login form if present.
 *
 * The sangraha dashboard login page asks for a server URL and then calls
 * GET /admin/v1/health to verify connectivity. Tests that mock the API must
 * intercept that call before invoking this helper so that the "Connect" button
 * succeeds.
 */
export async function loginAsDev(page: Page) {
  await page.goto('/')

  // The app starts on the Login page which asks for a Server URL.
  const serverInput = page.locator('input[type="text"]').first()
  const isLoginPage = await serverInput.isVisible({ timeout: 3000 }).catch(() => false)

  if (isLoginPage) {
    const adminURL = process.env.SANGRAHA_ADMIN_URL ?? 'http://localhost:9001'
    await serverInput.fill(adminURL)
    await page.click('button[type="submit"]')
    // Wait until we are past the login screen (shell nav becomes visible)
    await page.waitForSelector('nav', { timeout: 10_000 })
  }
}
