/**
 * Playwright tests for the Users page.
 *
 * All admin API calls are intercepted with page.route() so the tests run
 * without a live sangraha binary.  The Vite dev server (or the built assets)
 * must be reachable at SANGRAHA_ADMIN_URL (default http://localhost:9001).
 */

import { test, expect, Page } from '@playwright/test'
import { loginAsDev } from './helpers/auth'

// ---------------------------------------------------------------------------
// Mock data
// ---------------------------------------------------------------------------

const mockUsers = [
  { access_key: 'AKTEST001', owner: 'alice', is_root: true },
  { access_key: 'AKTEST002', owner: 'bob', is_root: false },
]

const createdUserResponse = {
  access_key: 'AKNEWUSER',
  owner: 'charlie',
  is_root: false,
  secret_key: 'supersecretvalue1234',
}

const rotatedKeyResponse = {
  access_key: 'AKTEST002',
  owner: 'bob',
  is_root: false,
  secret_key: 'newrotatedsecret5678',
}

// ---------------------------------------------------------------------------
// Shared mock setup
// ---------------------------------------------------------------------------

async function setupMocks(page: Page) {
  // Health endpoint — used by Login page to verify connectivity.
  await page.route('**/admin/v1/health', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ status: 'ok' }),
    }),
  )

  // Info endpoint — polled by the Shell sidebar.
  await page.route('**/admin/v1/info', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ version: 'test', build_time: '2026-01-01', uptime_sec: 3600 }),
    }),
  )

  // Users list and create.
  await page.route('**/admin/v1/users', route => {
    if (route.request().method() === 'POST') {
      return route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify(createdUserResponse),
      })
    }
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockUsers),
    })
  })

  // Per-user routes: DELETE and rotate-key.
  await page.route('**/admin/v1/users/**', route => {
    const method = route.request().method()
    const url = route.request().url()

    if (method === 'DELETE') {
      return route.fulfill({ status: 204, body: '' })
    }

    // POST to .../keys/rotate
    if (method === 'POST' && url.includes('keys/rotate')) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(rotatedKeyResponse),
      })
    }

    // Fallthrough — shouldn't happen in these tests.
    return route.fulfill({ status: 404, body: '' })
  })

  // Buckets list — needed by Overview stats card.
  await page.route('**/admin/v1/buckets', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([]),
    }),
  )

  // Connections — needed by Overview / Monitoring.
  await page.route('**/admin/v1/connections', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ active_connections: 0 }),
    }),
  )

  // TLS info.
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

  // Live log stream.
  await page.route('**/admin/v1/logs/stream**', route =>
    route.fulfill({
      status: 200,
      contentType: 'text/event-stream',
      headers: { 'Cache-Control': 'no-cache', Connection: 'keep-alive' },
      body: '',
    }),
  )
}

async function goToUsers(page: Page) {
  await setupMocks(page)
  await loginAsDev(page)
  await page.click('button:has-text("Users")')
  await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Users flows', () => {
  test('Users page heading renders', async ({ page }) => {
    await goToUsers(page)
    await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible()
  })

  test('User list table renders with Access Key and Owner columns', async ({ page }) => {
    await goToUsers(page)

    await expect(page.getByText('Access Key')).toBeVisible()
    await expect(page.getByText('Owner')).toBeVisible()
    await expect(page.getByText('Role')).toBeVisible()
  })

  test('Mocked user rows are visible in the table', async ({ page }) => {
    await goToUsers(page)

    await expect(page.getByText('AKTEST001')).toBeVisible()
    await expect(page.getByText('alice')).toBeVisible()
    await expect(page.getByText('AKTEST002')).toBeVisible()
    await expect(page.getByText('bob')).toBeVisible()
  })

  test('Root user shows "root" role badge', async ({ page }) => {
    await goToUsers(page)

    // alice is_root: true should have a "root" badge
    await expect(page.getByText('root')).toBeVisible()
  })

  test('Non-root user shows "user" role badge', async ({ page }) => {
    await goToUsers(page)

    // bob is_root: false should have a "user" badge
    await expect(page.getByText('user')).toBeVisible()
  })

  test('Owner name input and Create User button are present', async ({ page }) => {
    await goToUsers(page)

    await expect(page.locator('input[placeholder="Owner name…"]')).toBeVisible()
    await expect(page.getByRole('button', { name: /Create User/ })).toBeVisible()
  })

  test('Create User button is disabled when owner input is empty', async ({ page }) => {
    await goToUsers(page)

    const createBtn = page.getByRole('button', { name: /Create User/ })
    await expect(createBtn).toBeDisabled()
  })

  test('Create User button enables after typing an owner name', async ({ page }) => {
    await goToUsers(page)

    await page.fill('input[placeholder="Owner name…"]', 'charlie')
    const createBtn = page.getByRole('button', { name: /Create User/ })
    await expect(createBtn).toBeEnabled()
  })

  test('Creating a user shows the new access key and secret key', async ({ page }) => {
    await goToUsers(page)

    await page.fill('input[placeholder="Owner name…"]', 'charlie')
    await page.click('button:has-text("Create User")')

    // The success banner must show both keys from the mock response.
    await expect(page.getByText('User created!')).toBeVisible()
    await expect(page.getByText('AKNEWUSER')).toBeVisible()
    await expect(page.getByText('supersecretvalue1234')).toBeVisible()
  })

  test('New user success banner warns that secret key is shown once', async ({ page }) => {
    await goToUsers(page)

    await page.fill('input[placeholder="Owner name…"]', 'charlie')
    await page.click('button:has-text("Create User")')

    await expect(page.getByText('Save the secret key — it will not be shown again.')).toBeVisible()
  })

  test('Success banner can be dismissed', async ({ page }) => {
    await goToUsers(page)

    await page.fill('input[placeholder="Owner name…"]', 'charlie')
    await page.click('button:has-text("Create User")')

    await expect(page.getByText('User created!')).toBeVisible()
    await page.click('button:has-text("Dismiss")')

    await expect(page.getByText('User created!')).not.toBeVisible()
  })

  test('Rotate Key button triggers key rotation and shows new secret', async ({ page }) => {
    await goToUsers(page)

    // Click the "Rotate Key" button for the second user (bob).
    const rotateButtons = page.getByRole('button', { name: /Rotate Key/ })
    await rotateButtons.nth(1).click()

    // The success banner with rotated key info appears.
    await expect(page.getByText('Key rotated!')).toBeVisible()
    await expect(page.getByText('newrotatedsecret5678')).toBeVisible()
  })

  test('Delete button opens a confirmation modal', async ({ page }) => {
    await goToUsers(page)

    // Click the "Delete" button for the first user.
    const deleteButtons = page.getByRole('button', { name: /Delete/ })
    await deleteButtons.first().click()

    // Confirmation modal must appear.
    await expect(page.getByRole('heading', { name: 'Delete User' })).toBeVisible()
  })

  test('Delete confirmation modal shows the access key being deleted', async ({ page }) => {
    await goToUsers(page)

    await page.getByRole('button', { name: /Delete/ }).first().click()

    // Modal body should reference the access key.
    await expect(page.getByText('AKTEST001')).toBeVisible()
  })

  test('Delete confirmation modal has Cancel and Delete buttons', async ({ page }) => {
    await goToUsers(page)

    await page.getByRole('button', { name: /Delete/ }).first().click()

    // Both action buttons present inside the modal.
    const modal = page.locator('.fixed')
    await expect(modal.getByRole('button', { name: 'Delete' })).toBeVisible()
    await expect(modal.getByRole('button', { name: 'Cancel' })).toBeVisible()
  })

  test('Delete confirmation modal can be cancelled', async ({ page }) => {
    await goToUsers(page)

    await page.getByRole('button', { name: /Delete/ }).first().click()
    await expect(page.getByRole('heading', { name: 'Delete User' })).toBeVisible()

    await page.locator('.fixed').getByRole('button', { name: 'Cancel' }).click()

    // Modal must close.
    await expect(page.getByRole('heading', { name: 'Delete User' })).not.toBeVisible()
  })
})
