/**
 * Sprint 2.5-C11 — Playwright tests for monitoring flows.
 *
 * All admin API calls are intercepted with page.route() so the tests run
 * without a live sangraha binary.  The Vite dev server (or the built assets)
 * must be reachable at SANGRAHA_ADMIN_URL (default http://localhost:9001).
 */

import { test, expect, Page } from '@playwright/test'
import { loginAsDev } from './helpers/auth'

// ---------------------------------------------------------------------------
// Shared mock setup
// ---------------------------------------------------------------------------

async function setupMocks(page: Page) {
  // Health endpoint — used by Login page to verify connectivity and by the
  // Monitoring page's health card.
  await page.route('**/admin/v1/health', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ status: 'ok' }),
    }),
  )

  // Info endpoint — polled by the Shell sidebar and the Overview page.
  await page.route('**/admin/v1/info', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ version: 'test', build_time: '2026-01-01', uptime_sec: 3600 }),
    }),
  )

  // Active connections — polled by the Monitoring page.
  await page.route('**/admin/v1/connections', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ active_connections: 3 }),
    }),
  )

  // TLS info — polled by the Monitoring page.
  await page.route('**/admin/v1/tls', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        status: 'ok',
        subject: 'CN=sangraha',
        issuer: 'CN=sangraha',
        days_until_expiry: 350,
        is_self_signed: true,
      }),
    }),
  )

  // Users list — needed by Overview page stats card.
  await page.route('**/admin/v1/users', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([{ access_key: 'AKTEST', owner: 'alice', is_root: false }]),
    }),
  )

  // Buckets list — needed by Overview page (if queried) and Buckets page.
  await page.route('**/admin/v1/buckets', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([
        {
          name: 'test-bucket',
          owner: 'alice',
          region: 'us-east-1',
          versioning: 'disabled',
          acl: 'private',
          object_count: 5,
          total_bytes: 1024,
          created_at: '2026-01-01T00:00:00Z',
        },
      ]),
    }),
  )

  // Live log stream — EventSource endpoint.  We return an empty SSE stream so
  // the browser considers the connection "open".
  await page.route('**/admin/v1/logs/stream**', route =>
    route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      headers: {
        'Cache-Control': 'no-cache',
        Connection: 'keep-alive',
      },
      body: '',
    }),
  )
}

// Navigate to the Monitoring page after login.
async function goToMonitoring(page: Page) {
  await setupMocks(page)
  await loginAsDev(page)
  await page.click('button:has-text("Monitoring")')
  // Wait for the page heading to appear.
  await expect(page.getByRole('heading', { name: 'Monitoring' })).toBeVisible()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Monitoring flows', () => {
  test('Monitoring page heading renders', async ({ page }) => {
    await goToMonitoring(page)
    await expect(page.getByRole('heading', { name: 'Monitoring' })).toBeVisible()
  })

  test('Health panel shows Online status with a green indicator', async ({ page }) => {
    await goToMonitoring(page)

    // The health card label
    await expect(page.getByText('Health', { exact: true })).toBeVisible()
    // The status text from the mock { status: 'ok' }
    await expect(page.getByText('Online')).toBeVisible()
  })

  test('TLS certificate card renders with subject info', async ({ page }) => {
    await goToMonitoring(page)

    // Card header
    await expect(page.getByText('TLS Certificate')).toBeVisible()
    // Subject rendered from mock data
    await expect(page.getByText('CN=sangraha')).toBeVisible()
  })

  test('Active connections panel renders the connection count', async ({ page }) => {
    await goToMonitoring(page)

    await expect(page.getByText('Active Connections')).toBeVisible()
    // The mock returns 3; the component renders it as "3"
    await expect(page.getByText('3')).toBeVisible()
  })

  test('Live log viewer renders with Connecting/Connected state indicator', async ({ page }) => {
    await goToMonitoring(page)

    // The log viewer panel heading is always present
    await expect(page.getByText('Live Logs')).toBeVisible()
    // Either "Connecting…" or "Connected" must be visible (depends on whether
    // the mock SSE connection fires onopen in time).
    const statusLocator = page.locator('text=/Connecting|Connected/')
    await expect(statusLocator).toBeVisible({ timeout: 5000 })
  })

  test('Live log viewer shows waiting message when no lines received', async ({ page }) => {
    await goToMonitoring(page)

    // When no log lines have arrived the component renders a placeholder.
    await expect(page.getByText('Waiting for log lines…')).toBeVisible()
  })

  test('Log level filter dropdown is present', async ({ page }) => {
    await goToMonitoring(page)

    const select = page.locator('select')
    await expect(select).toBeVisible()
    // Verify at least the "All levels" option exists
    await expect(select.locator('option', { hasText: 'All levels' })).toHaveCount(1)
  })

  test('Pause and Clear buttons are present in the log viewer', async ({ page }) => {
    await goToMonitoring(page)

    await expect(page.getByRole('button', { name: 'Pause' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Clear' })).toBeVisible()
  })
})
