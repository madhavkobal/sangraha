/**
 * Sprint 2.5-D10 — Playwright tests for storage flows (Buckets & Objects pages).
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

const mockBuckets = [
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
]

const mockObjects = {
  objects: [
    {
      key: 'photos/cat.jpg',
      size: 204800,
      last_modified: '2026-01-15T10:00:00Z',
      etag: '"d41d8cd98f00b204e9800998ecf8427e"',
      content_type: 'image/jpeg',
    },
  ],
  prefixes: ['photos/'],
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

  // Buckets list.
  await page.route('**/admin/v1/buckets', route => {
    // Allow DELETE to pass through as a 204 (bucket deletion).
    if (route.request().method() === 'DELETE') {
      return route.fulfill({ status: 204, body: '' })
    }
    // Allow POST to pass through as a 201 (bucket creation).
    if (route.request().method() === 'POST') {
      return route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          name: 'new-bucket',
          owner: 'alice',
          region: 'us-east-1',
          versioning: 'disabled',
          acl: 'private',
          object_count: 0,
          total_bytes: 0,
          created_at: '2026-03-01T00:00:00Z',
        }),
      })
    }
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockBuckets),
    })
  })

  // Objects list for any bucket.
  await page.route('**/admin/v1/buckets/*/objects**', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(mockObjects),
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

  // Connections — needed by Overview / Monitoring.
  await page.route('**/admin/v1/connections', route =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ active_connections: 3 }),
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

async function goToBuckets(page: Page) {
  await setupMocks(page)
  await loginAsDev(page)
  await page.click('button:has-text("Buckets")')
  await expect(page.getByRole('heading', { name: 'Buckets' })).toBeVisible()
}

async function goToObjects(page: Page) {
  await setupMocks(page)
  await loginAsDev(page)
  // Navigate to buckets first then browse into the test bucket.
  await page.click('button:has-text("Buckets")')
  await expect(page.getByRole('heading', { name: 'Buckets' })).toBeVisible()
  // Click the folder/browse icon on the test-bucket row.
  await page.click('button[title="Browse objects"]')
  await expect(page.getByRole('heading', { name: 'Objects' })).toBeVisible()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Storage — Buckets flows', () => {
  test('Buckets page heading renders', async ({ page }) => {
    await goToBuckets(page)
    await expect(page.getByRole('heading', { name: 'Buckets' })).toBeVisible()
  })

  test('Bucket list renders the mocked bucket row', async ({ page }) => {
    await goToBuckets(page)

    // The mocked bucket name should appear in the table.
    await expect(page.getByText('test-bucket')).toBeVisible()
  })

  test('Bucket count subtitle is displayed', async ({ page }) => {
    await goToBuckets(page)
    // Subtitle shows "1 bucket" (singular) for one bucket.
    await expect(page.getByText(/1 bucket/)).toBeVisible()
  })

  test('Search input filters visible buckets', async ({ page }) => {
    await goToBuckets(page)

    const searchInput = page.locator('input[placeholder="Search buckets…"]')
    await expect(searchInput).toBeVisible()

    // Filtering by a non-matching term hides all rows.
    await searchInput.fill('no-such-bucket-xyz')
    await expect(page.getByText('No buckets found.')).toBeVisible()

    // Clearing the filter restores the list.
    await searchInput.clear()
    await expect(page.getByText('test-bucket')).toBeVisible()
  })

  test('New Bucket button opens Create Bucket dialog', async ({ page }) => {
    await goToBuckets(page)

    await page.click('button:has-text("New Bucket")')

    // Dialog heading appears.
    await expect(page.getByRole('heading', { name: 'Create Bucket' })).toBeVisible()
    // Input for bucket name is present.
    await expect(page.locator('input[placeholder="my-bucket"]')).toBeVisible()
  })

  test('Create Bucket dialog shows validation error for short name', async ({ page }) => {
    await goToBuckets(page)

    await page.click('button:has-text("New Bucket")')
    await page.fill('input[placeholder="my-bucket"]', 'ab') // only 2 chars — too short
    await page.click('button[type="submit"]')

    await expect(page.getByText('Name must be 3–63 characters')).toBeVisible()
  })

  test('Create Bucket dialog shows validation error for invalid chars', async ({ page }) => {
    await goToBuckets(page)

    await page.click('button:has-text("New Bucket")')
    await page.fill('input[placeholder="my-bucket"]', 'UPPER_CASE') // uppercase not allowed
    await page.click('button[type="submit"]')

    await expect(page.getByText('Name must be lowercase, alphanumeric, dots, or hyphens')).toBeVisible()
  })

  test('Create Bucket dialog can be cancelled', async ({ page }) => {
    await goToBuckets(page)

    await page.click('button:has-text("New Bucket")')
    await expect(page.getByRole('heading', { name: 'Create Bucket' })).toBeVisible()

    await page.click('button:has-text("Cancel")')

    // Dialog should be dismissed.
    await expect(page.getByRole('heading', { name: 'Create Bucket' })).not.toBeVisible()
  })

  test('Delete bucket opens confirmation dialog requiring bucket name', async ({ page }) => {
    await goToBuckets(page)

    // Click the delete (trash) icon on the test-bucket row.
    await page.click('button[title="Delete bucket"]')

    // Confirmation dialog appears.
    await expect(page.getByRole('heading', { name: 'Delete Bucket' })).toBeVisible()
    // The dialog mentions the bucket name (scope to modal to avoid matching the table row).
    await expect(page.locator('.fixed').getByText('test-bucket')).toBeVisible()

    // Delete button is disabled until the name is typed.
    const deleteBtn = page.locator('button:has-text("Delete"):not(:has-text("Cancel"))')
    await expect(deleteBtn).toBeDisabled()
  })

  test('Delete confirmation button enables after typing bucket name', async ({ page }) => {
    await goToBuckets(page)

    await page.click('button[title="Delete bucket"]')

    // Type the bucket name in the confirmation input.
    await page.fill('input[placeholder="test-bucket"]', 'test-bucket')

    const deleteBtn = page.locator('button:has-text("Delete"):not(:has-text("Cancel"))')
    await expect(deleteBtn).toBeEnabled()
  })
})

test.describe('Storage — Objects flows', () => {
  test('Objects page heading renders after browsing a bucket', async ({ page }) => {
    await goToObjects(page)
    await expect(page.getByRole('heading', { name: 'Objects' })).toBeVisible()
  })

  test('Breadcrumb shows the bucket name', async ({ page }) => {
    await goToObjects(page)

    // The bucket name appears as the first breadcrumb segment.
    await expect(page.getByRole('button', { name: 'test-bucket' })).toBeVisible()
  })

  test('Object table shows Name, Size, Last Modified columns', async ({ page }) => {
    await goToObjects(page)

    await expect(page.getByText('Name')).toBeVisible()
    await expect(page.getByText('Size')).toBeVisible()
    await expect(page.getByText('Last Modified')).toBeVisible()
  })

  test('Delete object opens confirmation dialog', async ({ page }) => {
    await goToObjects(page)

    // Click the trash icon on the first object row.
    await page.click('button[title="Delete"]')

    // Confirmation dialog appears.
    await expect(page.getByRole('heading', { name: 'Delete Object' })).toBeVisible()
  })

  test('Delete object confirmation requires typing the object name', async ({ page }) => {
    await goToObjects(page)

    await page.click('button[title="Delete"]')

    // The delete button starts disabled.
    const deleteBtn = page.locator('.fixed button:has-text("Delete")')
    await expect(deleteBtn).toBeDisabled()
  })

  test('Objects page shows "No objects in this prefix" when list is empty', async ({ page }) => {
    // Override the objects mock to return an empty list.
    await setupMocks(page)
    await page.route('**/admin/v1/buckets/*/objects**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ objects: [], prefixes: [] }),
      }),
    )
    await loginAsDev(page)
    await page.click('button:has-text("Buckets")')
    await page.click('button[title="Browse objects"]')

    await expect(page.getByText('No objects in this prefix.')).toBeVisible()
  })
})
